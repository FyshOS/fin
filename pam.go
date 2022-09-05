package main

/*
#cgo LDFLAGS: -lpam
#cgo openbsd CFLAGS: -I/usr/local/include
<<<<<<< HEAD
#cgo openbsd LDFLAGS: -L/usr/local//lib
=======
#cgo openbsd LDFLAGS: -L/usr/local/lib
>>>>>>> fac6a73 (Addressing feedback)

#include <stdbool.h>
#include <stdlib.h>
#include <unistd.h>

char *homedir(const char *username);
bool login(const char *username, const char *password, const char *exec, pid_t *child_pid);
bool logout(void);
*/
import "C"
import (
	"errors"
)

// homedir gets the home directory for a username.
// An error is returned if the lookup was not successful.
func homedir(username string) (string, error) {
	cName := C.CString(username)
	cHome := C.homedir(cName)
	if cHome == nil {
		return "", errors.New("unable to look up homedir")
	}
	return C.GoString(cHome), nil
}

// login logs in the username with password and returns the pid of the login process
// or an error if login failed
func login(username, password, exec string) (int, error) {
	cUser := C.CString(username)
	cPass := C.CString(password)
	cExec := C.CString("exec " + exec)

	var child C.pid_t
	ok := bool(C.login(cUser, cPass, cExec, &child))
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
