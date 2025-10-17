package provider

import "testing"

func TestStreamResource_New(t *testing.T) {
	r := NewStreamResource()
	if r == nil { t.Fatal("nil") }
}
