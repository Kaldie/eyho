package services

import "github.com/eyho/eyho/src/auth"

func authClaims(name, email string) auth.Claims { return auth.Claims{Name: name, Email: email} }
