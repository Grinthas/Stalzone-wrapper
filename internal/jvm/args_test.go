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
