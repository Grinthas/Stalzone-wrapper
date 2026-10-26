package tunnel

import (
	"encoding/binary"
	"net"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestFlags(t *testing.T) {
	got := Flags(map[string]string{"eu": "1.2.3.4:7000", "ru": "bad", "na": ""})
	want := []string{"-Droxy_address_override.eu=1.2.3.4:7000"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestProbe(t *testing.T) {
	// Фейковый узел: отвечает на порту +1 тем же UUID, задержкой сервера 42 мс и флагом лимита.
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	go func() {
		buf := make([]byte, 64)
		n, from, err := pc.ReadFrom(buf)
		if err != nil || n != 16 {
			return
		}
		resp := make([]byte, 21)
		copy(resp, buf[:16])
		binary.BigEndian.PutUint32(resp[16:20], 42)
		resp[20] = 1
		pc.WriteTo(resp, from)
	}()
	port := pc.LocalAddr().(*net.UDPAddr).Port
	r := Probe(net.JoinHostPort("127.0.0.1", strconv.Itoa(port-1)), time.Second)
	if r.Error != "" || r.ServerMs != 42 || !r.Limited {
		t.Fatalf("unexpected %+v", r)
	}
}
