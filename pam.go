package main

/*
#cgo LDFLAGS: -lpam

#include <stdbool.h>
#include <stdlib.h>

bool login(const char *username, const char *password, pid_t *child_pid);
bool logout(void);
*/
import "C"
import (
	"errors"
)

// login logs in the username with password and returns the pid of the login process
// or an error if login failed
func login(username, password string) (int, error) {
	cUser := C.CString(username)
	cPass := C.CString(password)

	var child C.pid_t
	ok := bool(C.login(cUser, cPass, &child))
	if !ok {
		return 0, errors.New("could not log in user")
	}
	return int(child), nil
}

// logout requests the user log out and returns an error if this was not possible
func logout() error {
	if !bool(C.logout()) {
		return errors.New("could not log user out")
	}

	return nil
}
