// The code in this file is from the excellent blog post https://www.gulshansingh.com/posts/how-to-write-a-display-manager/

#include <security/pam_appl.h>
#ifdef __FreeBSD__
#include <security/openpam.h>
#else
#include <security/pam_misc.h>
#endif

#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <pwd.h>
#include <paths.h>
#include <unistd.h>
#include <sys/types.h>
#include <grp.h>

#define SERVICE_NAME "display_manager"

#define err(name)                                   \
    do {                                            \
        fprintf(stderr, "%s: %s\n", name,           \
                pam_strerror(pam_handle, result));  \
        end(result);                                \
        return false;                               \
    } while (1);                                    \

static pam_handle_t *pam_handle;

static void change_identity (struct passwd *pw) {
	if (initgroups(pw->pw_name, pw->pw_gid) == -1)
		exit(1);
	endgrent();
	if (setgid(pw->pw_gid) || setuid(pw->pw_uid))
		exit(1);
}


static int end(int last_result) {
    int result = pam_end(pam_handle, last_result);
    pam_handle = 0;
    return result;
}

static int conv(int num_msg, const struct pam_message **msg,
                struct pam_response **resp, void *appdata_ptr) {
    int i;

    *resp = calloc(num_msg, sizeof(struct pam_response));
    if (*resp == NULL) {
        return PAM_BUF_ERR;
    }

    int result = PAM_SUCCESS;
    for (i = 0; i < num_msg; i++) {
        char *username, *password;
        switch (msg[i]->msg_style) {
        case PAM_PROMPT_ECHO_ON:
            username = ((char **) appdata_ptr)[0];
            (*resp)[i].resp = strdup(username);
            break;
        case PAM_PROMPT_ECHO_OFF:
            password = ((char **) appdata_ptr)[1];
            (*resp)[i].resp = strdup(password);
            break;
        case PAM_ERROR_MSG:
            fprintf(stderr, "%s\n", msg[i]->msg);
            result = PAM_CONV_ERR;
            break;
        case PAM_TEXT_INFO:
            printf("%s\n", msg[i]->msg);
            break;
        }
        if (result != PAM_SUCCESS) {
            break;
        }
    }

    if (result != PAM_SUCCESS) {
        free(*resp);
        *resp = 0;
    }

    return result;
}

static void set_env(char *name, char *value) {
    // The `+ 2` is for the '=' and the null byte
    size_t name_value_len = strlen(name) + strlen(value) + 2;
    char *name_value = malloc(name_value_len);
    snprintf(name_value, name_value_len,  "%s=%s", name, value);
    pam_putenv(pam_handle, name_value);
    free(name_value);
}

static void init_env(struct passwd *pw) {
    set_env("HOME", pw->pw_dir);
    set_env("PWD", pw->pw_dir);
    set_env("SHELL", pw->pw_shell);
    set_env("USER", pw->pw_name);
    set_env("LOGNAME", pw->pw_name);
    set_env("PATH", "/usr/local/sbin:/usr/local/bin:/usr/bin");
    set_env("MAIL", _PATH_MAILDIR);
    set_env("DISPLAY", ":0");

    size_t xauthority_len = strlen(pw->pw_dir) + strlen("/.Xauthority") + 1;
    char *xauthority = malloc(xauthority_len);
    snprintf(xauthority, xauthority_len, "%s/.Xauthority", pw->pw_dir);
    set_env("XAUTHORITY", xauthority);
    free(xauthority);
}

char *homedir(const char *username) {
    struct passwd *pw = getpwnam(username);
    if (pw == NULL) {
        return NULL;
    }
    return pw->pw_dir;
}

bool login(const char *username, const char *password, const char *exec, pid_t *child_pid) {
    const char *data[2] = {username, password};
    struct pam_conv pam_conv = {
        conv, data
    };
    setenv("XDG_SESSION_TYPE", "x11", 1);

    int result = pam_start(SERVICE_NAME, username, &pam_conv, &pam_handle);
    if (result != PAM_SUCCESS) {
        err("pam_start");
    }

    result = pam_authenticate(pam_handle, 0);
    if (result != PAM_SUCCESS) {
        err("pam_authenticate");
    }

    result = pam_acct_mgmt(pam_handle, 0);
    if (result != PAM_SUCCESS) {
        err("pam_acct_mgmt");
    }

    result = pam_setcred(pam_handle, PAM_ESTABLISH_CRED);
    if (result != PAM_SUCCESS) {
        err("pam_setcred");
    }

    struct passwd *pw = getpwnam(username);
    init_env(pw);

    result = pam_open_session(pam_handle, 0);
    if (result != PAM_SUCCESS) {
        pam_setcred(pam_handle, PAM_DELETE_CRED);
        err("pam_open_session");
    }

    *child_pid = fork();
    if (*child_pid == 0) {
		change_identity(pw);
        chdir(pw->pw_dir);
        char **env = pam_getenvlist(pam_handle);
        execle(pw->pw_shell, pw->pw_shell, "-c", exec, NULL, env);
        printf("Failed to start window manager");
        exit(1);
    }

    return true;
}



bool logout(void) {
	int result = pam_close_session(pam_handle, 0);
	if (result != PAM_SUCCESS) {
		pam_setcred(pam_handle, PAM_DELETE_CRED);
		err("pam_close_session");
	}

	result = pam_setcred(pam_handle, PAM_DELETE_CRED);
	if (result != PAM_SUCCESS) {
		err("pam_setcred");
	}

	end(result);
	return true;
}
