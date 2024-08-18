package authentication

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

const testVIN = "0123456789abcdefX"

func TestVerify(t *testing.T) {
	pkey := []byte{
		0x04, 0x77, 0x5e, 0x2e, 0xf5, 0x70, 0xd2, 0x92, 0xdf, 0x42, 0x4c, 0x09,
		0xf7, 0x0e, 0x7d, 0x95, 0x67, 0x8d, 0x5c, 0xe7, 0x81, 0x24, 0xac, 0xb3,
		0xf9, 0x37, 0x5b, 0x19, 0x8a, 0x4d, 0xa9, 0xf9, 0xe9, 0xf0, 0xcc, 0x6d,
		0x88, 0x0e, 0x2e, 0x60, 0x6e, 0x9b, 0x01, 0xb0, 0xfa, 0x62, 0xc6, 0x15,
		0x0e, 0x37, 0x1a, 0xa5, 0xd8, 0xf7, 0xf0, 0x9c, 0xc9, 0xe1, 0xbd, 0x3a,
		0x1b, 0x44, 0x98, 0x93, 0xec,
	}
	signedToken := "eyJhbGciOiJUZXNsYS5TUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOiJjb20udGVzbGEudmVoaWNsZS5NM1RFUkFTSElNQUJVQ0swMSIsImZvbyI6ImJhciIsImlzcyI6IkJIZGVMdlZ3MHBMZlFrd0o5dzU5bFdlTlhPZUJKS3l6K1RkYkdZcE5xZm5wOE14dGlBNHVZRzZiQWJENllzWVZEamNhcGRqMzhKeko0YjA2RzBTWWsrdz0ifQ.4Xq2tsynDYOhLBWpnqLJfsAzdOqTOwHkVowI47A-yzVXIQHaawObJvAYRoRs61oVwWiEQ7XYXG8WE_Vz_49eVirAV9NGKymj3HBTTjN5DmViWmFzfaIOXRWKJlE--vzU"
	token, err := jwt.Parse(signedToken, func(token *jwt.Token) (interface{}, error) { return pkey, nil })
	if err != nil {
		t.Fatal(err)
	}
	c, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("invalid claims type")
	}
	foo, ok := c["foo"]
	if !ok {
		t.Fatal("missing 'foo'")
	}
	if fooStr, ok := foo.(string); !ok || fooStr != "bar" {
		t.Fatalf("invalid type or value for foo: %v", foo)
	}
}
