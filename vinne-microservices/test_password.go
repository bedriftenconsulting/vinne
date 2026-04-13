package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := "Admin@123!"
	hash := "$2a$10$JZ3smvPJy4spBNVTp2kgZOWpKS2s0A0YUWOtJ3josvLezj0cCluFK"
	
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		fmt.Println("Password does NOT match:", err)
	} else {
		fmt.Println("Password matches!")
	}
}
