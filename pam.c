// The code in this file is from the excellent blog post https://www.gulshansingh.com/posts/how-to-write-a-display-manager/

#include <security/pam_appl.h>
#if defined(__FreeBSD__) || defined(__OpenBSD__) || defined(__APPLE__)
#include <security/openpam.h>
#else
#include <security/pam_misc.h>
#endif

#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <pwd.h>
#include <paths.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <grp.h>

#define SERVICE_NAME "fin"

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
        _Exit(1);
    endgrent();
    if (setgid(pw->pw_gid) || setuid(pw->pw_uid))
        _Exit(1);
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
    set_env("MAIL", _PATH_MAILDIR);
    set_env("DISPLAY", ":0");

    // Describe the graphical session to pam_systemd.
    // XDG_SEAT / XDG_VTNR are only present when fin owns the seat (see main.go);
    // in nested/embedded runs they are absent and we let logind decide.
    set_env("XDG_SESSION_TYPE", "x11");
    set_env("XDG_SESSION_CLASS", "user");
    char *seat = getenv("XDG_SEAT");
    if (seat && seat[0] != '\0') {
        set_env("XDG_SEAT", seat);
    }
    char *vtnr = getenv("XDG_VTNR");
    if (vtnr && vtnr[0] != '\0') {
        set_env("XDG_VTNR", vtnr);
    }

    #if !defined(__linux__)
    // On platforms without logind/pam_systemd nothing hands us a runtime dir,
    // so create a private per-user one and export XDG_RUNTIME_DIR ourselves.
    char runtime_dir[64];
    snprintf(runtime_dir, sizeof(runtime_dir), "/tmp/xdg-%u", (unsigned)pw->pw_uid);
    struct stat rst;
    int runtime_ok = 0;
    if (mkdir(runtime_dir, 0700) == 0) {
        if (chown(runtime_dir, pw->pw_uid, pw->pw_gid) == 0) {
            runtime_ok = 1;
        }
    } else if (lstat(runtime_dir, &rst) == 0 &&
               S_ISDIR(rst.st_mode) && rst.st_uid == pw->pw_uid) {
        // reuse the dir from an earlier login, but only while we still own it
        chmod(runtime_dir, 0700);
        runtime_ok = 1;
    }
    if (runtime_ok) {
        set_env("XDG_RUNTIME_DIR", runtime_dir);
    }
    #endif

    #if defined(__OpenBSD__)
    const char *path_def = "/bin:/bin:/sbin:/usr/bin:/usr/sbin:/usr/X11R6/bin:/usr/local/bin:/usr/local/sbin";
    size_t pathv_len = strlen(pw->pw_dir) + strlen(path_def) + 1;
    char *pathv = malloc(pathv_len);
    snprintf(pathv, pathv_len, "%s%s", pw->pw_dir, path_def);
    set_env("PATH", pathv);
    free(pathv);

    const char *kshrc = "/.kshrc";
    size_t env_len = strlen(pw->pw_dir) + strlen(kshrc) + 1;
    char *envv = malloc(env_len);
    snprintf(envv, env_len, "%s%s", pw->pw_dir, kshrc);
    set_env("ENV", envv);
    free(envv);

    #else
    set_env("PATH", "/usr/local/sbin:/usr/local/bin:/usr/bin");
    #endif

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

#define FINGERPRINT_SERVICE_NAME "fin-fingerprint"

// fconv handles fingerprint authentication via pam_fprintd, which only emits
// informational messages ("Swipe your finger...") and needs no input from us.
static int fconv(int num_msg, const struct pam_message **msg,
        struct pam_response **resp, void *appdata_ptr) {
    (void)msg;
    (void)appdata_ptr;
    *resp = calloc(num_msg, sizeof(struct pam_response));
    if (*resp == NULL) {
        return PAM_BUF_ERR;
    }
    return PAM_SUCCESS;
}

// perform_login runs the shared post-authentication session setup and launches
// the user's session. pam_handle must already be started and pam_authenticate
// must have succeeded; it is used by both password and fingerprint login.
static bool perform_login(const char *username, const char *exec, pid_t *child_pid) {
    int result;

    result = pam_acct_mgmt(pam_handle, 0);
    if (result != PAM_SUCCESS) {
        err("pam_acct_mgmt");
    }

    struct passwd *pw = getpwnam(username);
    init_env(pw);

    // Fork BEFORE opening the PAM session as pam_systemd records the process that
    // calls pam_open_session as the logind session leader.
    *child_pid = fork();
    if (*child_pid < 0) {
        err("fork");
    }
    if (*child_pid == 0) {
        setsid(); // become a session leader before logind registers us

        result = pam_setcred(pam_handle, PAM_ESTABLISH_CRED);
        if (result != PAM_SUCCESS) {
            fprintf(stderr, "pam_setcred: %s\n",
                    pam_strerror(pam_handle, result));
            _Exit(1);
        }

        result = pam_open_session(pam_handle, 0);
        if (result != PAM_SUCCESS) {
            fprintf(stderr, "pam_open_session: %s\n",
                    pam_strerror(pam_handle, result));
            pam_setcred(pam_handle, PAM_DELETE_CRED);
            _Exit(1);
        }

        change_identity(pw);
        chdir(pw->pw_dir);
        char **env = pam_getenvlist(pam_handle);
        execle(pw->pw_shell, pw->pw_shell, "-c", exec, NULL, env);
        printf("Failed to start window manager");
        _Exit(1);
    }

    return true;
}



bool login(const char *username, const char *password, const char *exec, pid_t *child_pid) {
    const char *data[2] = {username, password};
    struct pam_conv pam_conv = {
        conv, data
    };

    int result = pam_start(SERVICE_NAME, username, &pam_conv, &pam_handle);
    if (result != PAM_SUCCESS) {
        err("pam_start");
    }

    result = pam_authenticate(pam_handle, 0);
    if (result != PAM_SUCCESS) {
        err("pam_authenticate");
    }

    return perform_login(username, exec, child_pid);
}

// loginFingerprint authenticates via pam_fprintd (service fin-fingerprint) and,
// on a matching finger, opens the session exactly as password login does. It
// blocks in pam_authenticate until a finger is presented or the scan times out.
bool loginFingerprint(const char *username, const char *exec, pid_t *child_pid) {
    struct pam_conv pam_conv = {
        fconv, NULL
    };

    int result = pam_start(FINGERPRINT_SERVICE_NAME, username, &pam_conv, &pam_handle);
    if (result != PAM_SUCCESS) {
        err("pam_start");
    }

    result = pam_authenticate(pam_handle, 0);
    if (result != PAM_SUCCESS) {
        err("pam_authenticate");
    }

    return perform_login(username, exec, child_pid);
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
