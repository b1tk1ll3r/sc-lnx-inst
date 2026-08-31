# Omarchy Citizen als Git-Plugin veröffentlichen

Für automatische Plugin-Updates sollte Omarchy Citizen als normales öffentliches
Git-Repository veröffentlicht werden. `manifest.json` muss im Repository-Root liegen.

## Repository-Inhalt

Den kompletten Inhalt dieses Ordners in das Repository übernehmen, einschließlich:

```text
manifest.json
BarWidget.qml
Panel.qml
citizenctl
support-sanitize.py
backend/
```

Insbesondere sollte `backend/bin/omarchy-citizen-backend` mit eingecheckt werden, damit
Nutzer keine Go-Toolchain benötigen.

## Installation beim Nutzer

Sobald die Repository-URL feststeht:

```bash
omarchy plugin add https://github.com/DEIN-ACCOUNT/omarchy-citizen.git --enable --yes
```

Danach erkennt das Go-Backend `origin` automatisch. Im Plugin kann der Nutzer unter
**Erweiterte Optionen → Plugin-Updates → Auto-Updates einschalten** einmalig zustimmen.

Ab diesem Zeitpunkt prüft der systemd-User-Timer etwa alle sechs Stunden. Bei einem
Update ruft das Backend Omarchys eigenen nicht-interaktiven Plugin-Updater auf.

## Release-Ablauf

Vor jedem Commit/Release:

```bash
./tests/verify.sh
omarchy plugin validate .
```

Die GitHub-Actions-Datei `.github/workflows/ci.yml` führt dieselben Kernprüfungen automatisch bei Push und Pull Request aus.

Dann Versionsnummern in `manifest.json`, `citizenctl`, Go-Backend und Dokumentation
synchron halten, committen und pushen.

Tags/Releases sind für den Updater nicht zwingend erforderlich: Omarchy aktualisiert den
Git-Checkout per Fast-Forward auf dessen Upstream-Branch. Für nachvollziehbare Versionen
sind Git-Tags trotzdem empfehlenswert.
