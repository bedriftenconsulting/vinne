package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := "Test123!"
	
	// Generate hash
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Println("Error generating hash:", err)
		return
	}
	
	fmt.Println("Password:", password)
	fmt.Println("Hash:", string(hash))
	fmt.Println("Hash length:", len(string(hash)))
	
	// Test verification
	err = bcrypt.CompareHashAndPassword(hash, []byte(password))
	if err != nil {
		fmt.Println("Verification FAILED:", err)
	} else {
		fmt.Println("Verification SUCCESS!")
	}
}
