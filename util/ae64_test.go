package util

import (
	"fmt"
	"testing"
)

func TestAe64Encode(t *testing.T) {
	src := []byte("2MtvUt1KX5FQdJu1mCB9UvoYMYPNTJLcRQa")
	src2 := []byte("12345678")
	key := []byte("12345678")
	dst, _ := Ae64Encode(src, key)
	dst2, _ := Ae64Encode(src2, key)
	fmt.Println(dst)
	fmt.Println(dst2)
}

func TestAe64Decode(t *testing.T) {
	src := "akIwIjNtWSdAa1tVcXPIbuEvWfEXB4EVgP84snDK/UhbTjjTcOLQ3hWdpXiIBaSe"
	src2 := "m0lxCSrfYVhmOhZcOhICrw=="
	key := []byte("12345678")
	orgi, _ := Ae64Decode(src, key)
	orgi2, _ := Ae64Decode(src2, key)
	fmt.Println(string(orgi))
	fmt.Println(string(orgi2))
}
