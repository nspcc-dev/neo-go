package crypto

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	keyHex := "be85648006b70843c4f61a8ecac1584c1ab41b2e974de10817ed37d728e53760"
	keyBytes, _ := hex.DecodeString(keyHex)

	msg := "38907672f2f5678e5d5279694e7007183fadb4bcd69610ae87a73ff6633dedb6"

	//encrypted, err := AESEncrypt(keyBytes, msg)
	//if err != nil {
	//	t.Fatal(err)
	//}

	fmt.Println(encrypted)

	decrypted, err := AESDecrypt(keyBytes, encrypted)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(decrypted)

}
