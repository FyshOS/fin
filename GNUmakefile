# If PREFIX isn't provided, we check for /usr/local and use that if it exists.
# Otherwice we fall back to using /usr.

LOCAL != test -d $(DESTDIR)/usr/local && echo -n "/local" || echo -n ""
LOCAL ?= $(shell test -d $(DESTDIR)/usr/local && echo "/local" || echo "")
PREFIX ?= /usr$(LOCAL)

build:
	go build . || (echo "Failed to build fin"; exit 1)

install:
	install -Dm00755 fin $(DESTDIR)$(PREFIX)/bin/fin
	install -Dm00644 fin.service $(DESTDIR)$(PREFIX)/lib/systemd/system/fin.service

uninstall:
	-rm $(DESTDIR)$(PREFIX)/bin/fin
	-rm $(DESTDIR)$(PREFIX)/lib/systemd/system/fin.service

embed:
	DISPLAY=:0 Xephyr :1 -screen 1280x720 &
	DISPLAY=:1 go run .

# === FyshOS packagin ==========================================================
DEB_VERSION     ?=
DEB_NAME        ?= fin
DEB_SECTION     ?= x11
DEB_DESCRIPTION ?= FyshOS login manager
DEB_HOMEPAGE    ?= https://fyshos.com
DEB_SUDO        ?= -sudo
DEB_BUILD_DEPS  ?= libpam0g-dev libgl1-mesa-dev xorg-dev libwayland-dev \
                   libxkbcommon-dev

repo:
	fyshpkg make \
		-name "$(DEB_NAME)" \
		$(if $(DEB_VERSION),-version "$(DEB_VERSION)") \
		-section "$(DEB_SECTION)" \
		-description "$(DEB_DESCRIPTION)" \
		-homepage "$(DEB_HOMEPAGE)" \
		-build-deps "$(DEB_BUILD_DEPS)" \
		$(DEB_SUDO) $(FYSHPKG_FLAGS) \
		.

.PHONY: build install uninstall embed repo
