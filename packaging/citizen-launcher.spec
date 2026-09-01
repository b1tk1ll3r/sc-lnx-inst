Name:           citizen-launcher
Version:        %{cl_version}
Release:        1
Summary:        Star Citizen setup, launcher and self-maintaining gaming stack for Linux
License:        MIT
URL:            https://github.com/sendnwv/omarchy-sc
BuildArch:      x86_64
AutoReqProv:    no

Requires:       ca-certificates
Requires:       tar
Requires:       curl
Requires:       unzip
Requires:       xz
Requires:       polkit
Requires:       util-linux
Recommends:     zstd
Recommends:     pciutils
Recommends:     vulkan-tools
Recommends:     cabextract
Recommends:     xdg-utils

Source0:        citizen-launcher
Source1:        io.github.citizenlauncher.CitizenLauncher.desktop
Source2:        citizen-launcher.svg
Source3:        io.github.citizenlauncher.CitizenLauncher.metainfo.xml
Source4:        citizen-launcher-self-update.service
Source5:        citizen-launcher-self-update.timer
Source6:        citizen-launcher-migrate.desktop
Source7:        90-citizen-launcher.conf
Source8:        90-citizen-launcher-limits.conf
Source9:        release-repo

%description
Citizen Launcher manages a locally tested Wine runner, DXVK, RSI Launcher
compatibility, automatic maintenance, desktop integration and support
diagnostics for Star Citizen on Linux.

%prep

%build

%install
install -d %{buildroot}%{_bindir}
install -d %{buildroot}%{_datadir}/applications
install -d %{buildroot}%{_datadir}/icons/hicolor/scalable/apps
install -d %{buildroot}%{_datadir}/metainfo
install -d %{buildroot}%{_unitdir}
install -d %{buildroot}%{_prefix}/lib/sysctl.d
install -d %{buildroot}%{_sysconfdir}/security/limits.d
install -d %{buildroot}%{_sysconfdir}/xdg/autostart
install -d %{buildroot}%{_sysconfdir}/citizen-launcher
install -d %{buildroot}%{_licensedir}/%{name}
install -m755 %{SOURCE0} %{buildroot}%{_bindir}/citizen-launcher
install -m644 %{SOURCE1} %{buildroot}%{_datadir}/applications/io.github.citizenlauncher.CitizenLauncher.desktop
install -m644 %{SOURCE2} %{buildroot}%{_datadir}/icons/hicolor/scalable/apps/citizen-launcher.svg
install -m644 %{SOURCE3} %{buildroot}%{_datadir}/metainfo/io.github.citizenlauncher.CitizenLauncher.metainfo.xml
install -m644 %{SOURCE4} %{buildroot}%{_unitdir}/citizen-launcher-self-update.service
install -m644 %{SOURCE5} %{buildroot}%{_unitdir}/citizen-launcher-self-update.timer
install -m644 %{SOURCE6} %{buildroot}%{_sysconfdir}/xdg/autostart/citizen-launcher-migrate.desktop
install -m644 %{SOURCE7} %{buildroot}%{_prefix}/lib/sysctl.d/90-citizen-launcher.conf
install -m644 %{SOURCE8} %{buildroot}%{_sysconfdir}/security/limits.d/90-citizen-launcher.conf
install -m644 %{SOURCE9} %{buildroot}%{_sysconfdir}/citizen-launcher/release-repo
install -m644 %{_sourcedir}/LICENSE %{buildroot}%{_licensedir}/%{name}/LICENSE

%post
if command -v sysctl >/dev/null 2>&1; then
  sysctl -q -w vm.max_map_count=16777216 >/dev/null 2>&1 || true
fi
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload >/dev/null 2>&1 || true
  immutable=0
  [ -e /run/ostree-booted ] && immutable=1
  [ -e /run/transactional-update ] && immutable=1
  if [ "$immutable" -eq 0 ]; then
    systemctl enable --now citizen-launcher-self-update.timer >/dev/null 2>&1 || true
  fi
fi
if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database %{_datadir}/applications >/dev/null 2>&1 || true
fi

%preun
if [ "$1" -eq 0 ] && command -v systemctl >/dev/null 2>&1; then
  systemctl disable --now citizen-launcher-self-update.timer >/dev/null 2>&1 || true
fi

%postun
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload >/dev/null 2>&1 || true
fi

%files
%license %{_licensedir}/%{name}/LICENSE
%{_bindir}/citizen-launcher
%{_datadir}/applications/io.github.citizenlauncher.CitizenLauncher.desktop
%{_datadir}/icons/hicolor/scalable/apps/citizen-launcher.svg
%{_datadir}/metainfo/io.github.citizenlauncher.CitizenLauncher.metainfo.xml
%{_unitdir}/citizen-launcher-self-update.service
%{_unitdir}/citizen-launcher-self-update.timer
%{_prefix}/lib/sysctl.d/90-citizen-launcher.conf
%{_sysconfdir}/security/limits.d/90-citizen-launcher.conf
%{_sysconfdir}/xdg/autostart/citizen-launcher-migrate.desktop
%config(noreplace) %{_sysconfdir}/citizen-launcher/release-repo

%changelog
* Tue Sep 01 2026 Citizen Launcher Project - 1.1.3-1
- Low-disk Gitea Fedora RPM build and release hardening
* Tue Sep 01 2026 Citizen Launcher Project - 1.1.1-1
- Multi-distribution native package support
