package jvm

import (
	"reflect"
	"testing"
)

func TestRewrite(t *testing.T) {
	in := []string{"-Xmx2G", "-XX:+UseParallelGC", "-Dfoo=bar", "-cp", "a.jar;b.jar", "net.game.Main", "-Xmx9G", "--user", "x"}
	got := Rewrite(in, []string{"-Xmx6G"})
	want := []string{"-Xmx6G", "-Dfoo=bar", "-cp", "a.jar;b.jar", "net.game.Main", "-Xmx9G", "--user", "x"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestRewriteJar(t *testing.T) {
	got := Rewrite([]string{"-Xms1G", "-jar", "game.jar", "-XX:keep"}, []string{"-Xms4G"})
	want := []string{"-Xms4G", "-jar", "game.jar", "-XX:keep"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestRewriteProperty(t *testing.T) {
	in := []string{"-Xmx4G", "-Droxy_address_override.eu=old:1", "-Dkeep=1", "Main"}
	got := Rewrite(in, []string{"-Droxy_address_override.eu=new:2"})
	want := []string{"-Droxy_address_override.eu=new:2", "-Xmx4G", "-Dkeep=1", "Main"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}
