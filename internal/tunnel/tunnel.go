// Package tunnel выбирает узел (Roxy-туннель), через который игра подключается
// к серверам региона. Игра поддерживает переопределение адреса параметром JVM
// -Droxy_address_override.<регион>=<host:port>.
//
// Протокол списка узлов и проверки задержки взят из MIT-проекта
// EXBO-Community/stalcraft-jvm-optimization.
package tunnel

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Login передаётся бэкенду в списке узлов как идентификатор клиента.
const Login = "StalzoneWrapper"

// Region — игровой регион.
type Region struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

var Regions = []Region{
	{"ru", "Россия"},
	{"eu", "Европа"},
	{"na", "Северная Америка"},
	{"sea", "Юго-Восточная Азия"},
	{"nea", "Северо-Восточная Азия"},
}

var sources = map[string][]string{
	"ru":  {"https://backend.stalzone.net/address_list", "https://backend.stalzone.com/address_list", "https://backend.stalcraftx.ru/address_list"},
	"eu":  {"https://backend-eu.stalzone.com/address_list", "https://backend-eu.stalzone.net/address_list"},
	"na":  {"https://backend-na.stalzone.com/address_list", "https://backend-na.stalzone.net/address_list"},
	"sea": {"https://backend-sea.stalzone.com/address_list", "https://backend-sea.stalzone.net/address_list"},
	"nea": {"https://backend-nea.stalzone.com/address_list", "https://backend-nea.stalzone.net/address_list"},
}

func ValidRegion(id string) bool {
	_, ok := sources[id]
	return ok
}

type Endpoint struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type Pool struct {
	Name    string     `json:"name"`
	Tunnels []Endpoint `json:"tunnels"`
}

type catalog struct {
	Mode  string `json:"mode"`
	Pools []Pool `json:"pools"`
}

// Fetch загружает список групп узлов региона, перебирая зеркала.
func Fetch(region string) ([]Pool, error) {
	list, ok := sources[region]
	if !ok {
		return nil, fmt.Errorf("неизвестный регион %q", region)
	}
	client := &http.Client{Timeout: 4 * time.Second}
	var errs []error
	for _, src := range list {
		u, _ := url.Parse(src)
		q := u.Query()
		q.Set("login", Login)
		u.RawQuery = q.Encode()
		pools, err := fetchOne(client, u.String())
		if err == nil {
			return pools, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", u.Host, err))
	}
	return nil, errors.Join(errs...)
}

func fetchOne(client *http.Client, u string) ([]Pool, error) {
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var c catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if c.Mode != "roxy" {
		return nil, fmt.Errorf("неподдерживаемый режим %q", c.Mode)
	}
	var pools []Pool
	for _, p := range c.Pools {
		var ok []Endpoint
		for _, e := range p.Tunnels {
			if _, err := probeAddress(e.Address); err == nil {
				ok = append(ok, e)
			}
		}
		if len(ok) > 0 {
			pools = append(pools, Pool{Name: p.Name, Tunnels: ok})
		}
	}
	if len(pools) == 0 {
		return nil, errors.New("пустой список узлов")
	}
	return pools, nil
}

// probeAddress — узел отвечает на проверку на порту +1 к игровому.
func probeAddress(address string) (string, error) {
	host, rawPort, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port >= 65535 || host == "" {
		return "", fmt.Errorf("неверный адрес %q", address)
	}
	return net.JoinHostPort(host, strconv.Itoa(port+1)), nil
}

// Result — итог проверки одного узла.
type Result struct {
	PingMs   int    `json:"pingMs"`   // задержка от компьютера до узла
	ServerMs int    `json:"serverMs"` // задержка от узла до игрового сервера
	Limited  bool   `json:"limited"`  // узел сообщил о лимите подключений
	Error    string `json:"error,omitempty"`
}

// Probe отправляет UDP-запрос с UUID и ждёт ответ с тем же UUID.
func Probe(address string, timeout time.Duration) Result {
	addr, err := probeAddress(address)
	if err != nil {
		return Result{Error: err.Error()}
	}
	token := make([]byte, 16)
	rand.Read(token)
	token[6] = token[6]&0x0f | 0x40
	token[8] = token[8]&0x3f | 0x80

	conn, err := net.DialTimeout("udp", addr, timeout)
	if err != nil {
		return Result{Error: "нет ответа"}
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))
	started := time.Now()
	if _, err := conn.Write(token); err != nil {
		return Result{Error: "нет ответа"}
	}
	buf := make([]byte, 1500)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return Result{Error: "нет ответа"}
		}
		if n != 21 || !bytes.Equal(buf[:16], token) {
			continue
		}
		server := int32(binary.BigEndian.Uint32(buf[16:20]))
		return Result{
			PingMs:   int(time.Since(started).Milliseconds()),
			ServerMs: int(max(server, 0)),
			Limited:  buf[20] != 0,
		}
	}
}

// ProbeAll проверяет узлы параллельно (не больше 16 одновременно), по 3 попытки,
// и возвращает лучший результат для каждого адреса.
func ProbeAll(addresses []string) map[string]Result {
	out := make(map[string]Result, len(addresses))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	for _, a := range addresses {
		wg.Add(1)
		go func(a string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			best := Result{Error: "нет ответа"}
			for i := 0; i < 3; i++ {
				r := Probe(a, time.Second)
				if r.Error == "" && (best.Error != "" || r.PingMs+r.ServerMs < best.PingMs+best.ServerMs) {
					best = r
				}
			}
			mu.Lock()
			out[a] = best
			mu.Unlock()
		}(a)
	}
	wg.Wait()
	return out
}

// Flags возвращает параметры JVM для выбранных узлов (регион → адрес).
func Flags(overrides map[string]string) []string {
	var flags []string
	for _, r := range Regions {
		if a := overrides[r.ID]; a != "" {
			if _, err := probeAddress(a); err == nil {
				flags = append(flags, fmt.Sprintf("-Droxy_address_override.%s=%s", r.ID, a))
			}
		}
	}
	return flags
}
