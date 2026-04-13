package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	hash := "$2a$10$UB5AlFDtnl.qNX7ug0YR.u/oijr41GonPvWA1AApjSarBye4PQ.H2"
	password := "123456"
	
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		fmt.Printf("Password does NOT match: %v\n", err)
	} else {
		fmt.Println("Password matches!")
	}
}
