Name: citizen-launcher
Version: 0.9.1
Release: 1%{?dist}
Summary: Star Citizen launcher and self-maintaining Wine stack for Linux
License: MIT
BuildArch: x86_64
Requires: ca-certificates, tar, xdg-utils

%description
Distro-neutral Citizen Launcher with Wine, DXVK, RSI Launcher management and local GUI.

%install
mkdir -p %{buildroot}%{_bindir}
install -m755 citizen-launcher %{buildroot}%{_bindir}/citizen-launcher

%files
%{_bindir}/citizen-launcher
