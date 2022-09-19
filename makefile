# OpenBSD makefile

# If PREFIX isn't provided, we check for /usr/local and use that if it exists.
# Otherwice we fall back to using /usr.

LOCAL != test -d $(DESTDIR)/usr/local && echo -n "/local" || echo -n ""
LOCAL ?= $(shell test -d $(DESTDIR)/usr/local && echo "/local" || echo "")
PREFIX ?= /usr$(LOCAL)

build:
	go build . || (echo "Failed to build fin"; exit 1)

install:
	install -D -g bin fin $(DESTDIR)$(PREFIX)/bin/fin
	install -Dm00555 fin.rc /etc/rc.d/fin

uninstall:
	rm $(DESTDIR)$(PREFIX)/bin/fin
	rm /etc/rc.d/fin

embed:
	go build .
	Xephyr :1 -screen 1280x720 &
	DISPLAY=:1 doas ./fin
