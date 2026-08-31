# Omarchy Citizen 0.8.0 – Rundum-Sorglos

## Installation

Entpacken und einmal ausführen:

```bash
./INSTALLIEREN.sh
```

oder:

```bash
./install.sh
```

Danach übernimmt **Autopilot**.

## Was automatisch aktuell gehalten wird

Ohne weitere Pflege des Nutzers:

- Omarchy Citizen selbst (wenn als Git-Plugin installiert)
- offizieller LUG Helper als AppImage
- stabiler LUG-Wine-Runner
- DXVK
- Auswahl des verwalteten Wine-Runners im Start-Script

Der Wartungstimer läuft alle sechs Stunden.

## Sicherheitsmechanismus für Wine Updates

Ein neuer Wine-Runner wird nicht einfach installiert.

Omarchy Citizen baut zuerst einen temporären Wine-Prefix und verlangt erfolgreich:

```text
wineboot
drive_c
system.reg
user.reg
%APPDATA%
```

Erst danach wird der Runner aktiviert.

Damit hätte der Fehler, den wir während der Entwicklung gesehen haben
(`drive_c` fehlt / `%AppData% returned empty string`), bereits *vor* der echten
Star-Citizen-Installation den Autopilot-Test gestoppt.

## Warum kein AUR-LUG-Paket mehr?

Der offizielle LUG Helper wird als AppImage verwaltet. Dadurch hängt der
Star-Citizen-Workflow nicht mehr davon ab, ob das AUR-Paket, dessen Winetricks
oder dessen GUI-Abhängigkeiten gerade passend installiert sind.

Bestehende AUR-LUG-Installationen dürfen vorhanden bleiben; Omarchy Citizen
bevorzugt jedoch seine eigene verwaltete AppImage-Kopie.

## Systemupdates

Omarchy Citizen installiert absichtlich keine dauerhafte passwortlose
Pacman-/sudo-Regel.

Omarchy-System-, Kernel- und GPU-Treiberupdates bleiben bei:

```bash
omarchy update -y
```

Das ist der von Omarchy vorgesehene vollständige Updatepfad. Die Star-Citizen-
Komponenten selbst werden davon unabhängig durch Autopilot gepflegt.

## Support

Bei einem Fehler:

**✦SC → Support-Paket erstellen**

Das Paket enthält zusätzlich:

- Autopilot-Status
- verwaltete LUG-/Wine-/DXVK-Versionen
- Wine-Selbsttest
- Maintenance-Ergebnis
- Updater-Log

## Wine-Kompatibilitätsfallback (0.8.0)

Autopilot verwendet nicht blind die höchste Versionsnummer.

Beispiel:

```text
11.16-1 testen → nicht kompatibel
11.15-1 testen → nicht kompatibel
11.14-1 testen → funktioniert
→ 11.14-1 wird automatisch aktiviert
```

Der Nutzer muss keine Wine-Version auswählen.

Die konkrete Ursache eines fehlgeschlagenen Tests wird jetzt vollständig ins
Updater-/Support-Log geschrieben, auch wenn `wineboot` selbst keine Textausgabe
liefert.
