# Omarchy Citizen 0.6.2 – Plug-&-Play-Installation

## Ziel

Der Nutzer soll Star Citizen unter Omarchy benutzen können, ohne Linux administrieren zu müssen.

Im normalen Alltag sind nur drei Dinge wichtig:

1. **EINRICHTEN & STARTKLAR MACHEN** bzw. **STAR CITIZEN STARTEN**
2. **Problem beheben**
3. **Support-Paket erstellen**

Wine-Runner, DXVK, einzelne Reparaturschritte, Logs und NGL befinden sich unter
**Erweiterte Optionen**.

---

## 1. Plugin installieren

### Mausfreundlich

Entpacke `omarchy-citizen-0.6.0.zip`.

Im entpackten Ordner liegt:

`INSTALLIEREN.sh`

Führe diese Datei über deinen Dateimanager als Programm aus.

Falls der Dateimanager Skripte nicht direkt ausführt, bleibt der Terminal-Fallback:

```bash
./INSTALLIEREN.sh
```

oder:

```bash
./install.sh
```

Der Plugin-Installer benötigt kein `sudo` und schreibt nur nach:

```text
~/.config/omarchy/plugins/local.omarchy-citizen/
```

Nach der Installation erscheint **✦SC** in der Omarchy-Leiste.

---

## 2. Erstes Setup

Klicke:

**✦SC → EINRICHTEN & STARTKLAR MACHEN**

Omarchy Citizen übernimmt danach soweit möglich selbst:

- benötigte Systempakete prüfen/installieren
- `zenity`, `polkit`, `cabextract`, `unzip`
- LUG Helper aus dem AUR installieren
- LUG Preflight Check starten
- Star-Citizen-/RSI-Launcher-Einrichtung starten
- vorhandenen Wine-Prefix erkennen
- Start-Script erkennen/reparieren

Für Paketinstallationen kann einmal ein sichtbares Omarchy-Terminal erscheinen.
Das ist absichtlich so, damit Passwortabfragen sicher und nachvollziehbar bleiben.
Der Nutzer muss dort keine Paketnamen oder Linux-Befehle kennen.

---

## 3. Normaler Alltag

Wenn das Panel **FLIGHT READY** zeigt:

**✦SC → STAR CITIZEN STARTEN**

Das Plugin verwendet weiterhin das vom LUG Helper verwaltete `sc-launch.sh`.

---

## 4. Problem beheben

Klicke:

**✦SC → Problem beheben**

Der Assistent prüft zuerst automatisch, ob etwas offensichtlich fehlt.

Wenn das System grundsätzlich eingerichtet ist, fragt er nur nach dem Symptom:

- Automatisch prüfen
- RSI Launcher startet nicht / ist kaputt
- Star Citizen startet nicht
- Grafik / Performance / Abstürze
- NGL macht Probleme
- Problem unklar

Dahinter werden die passenden LUG-Helper-Funktionen aufgerufen.

Der Nutzer muss keine Begriffe wie Wine-Prefix, DXVK oder `vm.max_map_count` kennen.

---

## 5. Permanentes Debug-Log

Omarchy Citizen protokolliert seine eigenen Aktionen dauerhaft unter:

```text
~/.local/state/omarchy-citizen/omarchy-citizen.log
```

Darin stehen unter anderem:

- Plugin-Version
- aufgerufene Plugin-Aktionen
- Start-/Reparaturvorgänge
- LUG-Helper-Aufrufe
- Setup-/Update-Abläufe
- Fehlermeldungen des Plugins

Das lokale Roh-Log kann in **Erweiterte Optionen → Plugin-Debuglog öffnen**
angesehen werden.

Das Roh-Log wird nicht automatisch hochgeladen oder versendet. Ab ungefähr 4 MiB wird es automatisch gekürzt; die neuesten ungefähr 2 MiB bleiben erhalten.

---

## 6. Support-Paket für Nutzer

Klicke:

**✦SC → Support-Paket erstellen**

Das Plugin erzeugt automatisch eine Datei wie:

```text
~/Downloads/Omarchy-Citizen-Support-OC-20260831-123456-abcdef.tar.gz
```

Zusätzlich wird eine **Support-ID** angezeigt.

### Standardmäßig enthalten

- Omarchy Citizen Status
- Plugin-Version
- `omarchy debug --no-sudo --print`
- Kernel/Desktop-Informationen
- GPU/Vulkan-Zusammenfassung
- RAM
- Dateisystemtyp und freier Speicher
- relevante Paketversionen
- Star-Citizen-Kernel-Limits
- Plugin-Validierung
- letzte Omarchy-Citizen-Debugmeldungen
- letzter Omarchy-Update-Logauszug, falls vorhanden
- Star-Citizen-/LUG-Pfadstatus
- begrenzter `sc-launch.log`-Auszug
- bereinigte Kopie des LUG-Start-Scripts
- Go-Updater-Status (Git/ZIP, Auto-Update, Commit-Stand)
- bereinigter Updater-Logauszug

### Nicht standardmäßig enthalten

- Browserdaten
- SSH-Schlüssel
- Passwortdateien
- vollständige Star-Citizen-Game.log-Dateien
- vollständige EAC-Logs
- Dateien aus dem RSI-Account/Browserprofil

### Automatische Maskierung beim Export

Das Support-Paket versucht folgende Daten zu maskieren:

- Home-Pfad
- Benutzername
- Hostname
- E-Mail-Adressen
- IPv4-Adressen
- MAC-Adressen
- Bearer-/JWT-Tokens
- typische `token`, `session`, `cookie`, `password`, `secret`-Werte
- typische Auth-Queryparameter in URLs

Die Maskierung ist eine Best-Effort-Funktion. Nutzer können das Archiv vor dem
Versenden selbst öffnen und prüfen.

---

## 7. Updates

### Plugin selbst

0.6.0 enthält ein statisch gebautes Go-Backend. Es erkennt automatisch, ob Omarchy
Citizen als Git-Plugin oder aus einem ZIP installiert wurde.

Bei einer Git-Installation gibt es unter **Erweiterte Optionen → Plugin-Updates**:

- **Plugin-Update prüfen**
- **Plugin jetzt aktualisieren**
- **Auto-Updates einschalten/ausschalten**
- **Updater-Log öffnen**

Beim Aktivieren von Auto-Updates wird das aktuelle Git-`origin` als vertrauenswürdige
Quelle gespeichert. Ein systemd-User-Timer prüft ungefähr alle sechs Stunden mit einer
zufälligen Verzögerung von bis zu 15 Minuten.

Ein automatisches Update wird verweigert, wenn:

- das Git-Remote nachträglich geändert wurde
- lokale Änderungen im Plugin-Checkout liegen
- der lokale Branch vom Upstream abgewichen ist
- bereits ein anderes Plugin-Update läuft
- Omarchys Plugin-Validierung fehlschlägt

Das Backend baut keinen eigenen Plugin-Paketmanager. Die eigentliche Aktualisierung läuft
über Omarchys offiziellen nicht-interaktiven Weg:

```bash
omarchy plugin update local.omarchy-citizen --yes
```

Dadurch bleiben Fast-Forward-Prüfung, Validierung und Rollback bei Omarchy.

Bei der aktuellen ZIP-Installation zeigt das Backend bewusst **ZIP / nicht Git-verwaltet**.
Es lädt dann nichts aus einer erfundenen oder unbekannten Quelle herunter. Sobald das
Projekt als GitHub-/Git-Repository veröffentlicht und mit `omarchy plugin add` installiert
wird, erkennt das Backend `origin` automatisch.

### LUG und Omarchy

Zusätzlich befinden sich unter **Erweiterte Optionen**:

- LUG Update prüfen
- LUG aktualisieren
- Omarchy aktualisieren

Systempakete werden weiter über den normalen `omarchy update`-Weg aktualisiert.
Dadurch bleiben Omarchy-Migrationen und der vorgesehene Update-Ablauf erhalten.

### Updater-Dateien

```text
~/.config/omarchy-citizen/updater.json
~/.local/state/omarchy-citizen/updater.log
~/.local/lib/omarchy-citizen/omarchy-citizen-backend
```

Der Updater läuft nicht als dauerhafter Webserver. Der systemd-User-Timer startet ihn nur
für die jeweilige Prüfung.

---

## 8. NGL

NGL bleibt unter **Erweiterte Optionen** und ist deutlich als experimentell markiert.

Der normale Star-Citizen-Betrieb ist nicht von NGL abhängig.

---

## 9. Supportablauf für dich als Plugin-Betreuer

Wenn ein Nutzer ein Problem meldet, kannst du ihn bitten:

1. **✦SC** öffnen
2. **Support-Paket erstellen** klicken
3. dir die erzeugte `.tar.gz` schicken
4. die angezeigte Support-ID nennen

Damit brauchst du im ersten Supportkontakt normalerweise keine Terminalbefehle
vom Nutzer anzufordern.


## Hotfix 0.6.2 – Setup-Button

Version 0.6.2 ändert die Ausführung der Panel-Aktionen grundlegend.

Beim Klick auf **EINRICHTEN & STARTKLAR MACHEN** wird die Aktion nun direkt als
Quickshell-Prozess gestartet. Das Panel zeigt sofort **WIRD GESTARTET**.

Wenn der Start fehlschlägt, bleibt das Panel offen und zeigt den Fehler an.
Außerdem wird der fehlgeschlagene Terminalstart in
`~/.local/state/omarchy-citizen/omarchy-citizen.log` protokolliert und landet
im Support-Paket.

Nach der Installation wird der Omarchy-Shellprozess einmal neu gestartet, damit
die neue QML-Version sicher aktiv ist.
