import QtQuick
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Ui

Panel {
  id: root
  moduleName: "local.omarchy-citizen"
  manageIpc: false

  property var anchorItem: null
  property var hostWidget: null

  property string pluginVersion: "1.1.1"
  property string health: "checking"
  property string depsState: "checking"
  property string depsMissing: ""
  property string helperState: "checking"
  property string helperManaged: "missing"
  property string helperVersion: ""
  property string updateState: "unknown"
  property string updateVersion: ""
  property string prefixState: "checking"
  property string prefixDetail: ""
  property string launcherState: "checking"
  property string gameState: "checking"
  property bool checkingUpdate: false
  property bool advancedVisible: false
  property string backendState: "missing"
  property string backendVersion: ""
  property string pluginManaged: "manual"
  property string pluginAuto: "false"
  property string pluginUpdate: "unavailable"
  property string pluginLocalCommit: ""
  property string pluginRemoteCommit: ""
  property string autopilot: "false"
  property string managedLUGVersion: ""
  property string managedWineVersion: ""
  property string managedDXVKVersion: ""
  property string maintenanceResult: ""
  property string hardwareState: "unknown"
  property string hardwareReason: ""
  property string hardwareGPU: ""
  property string hardwareVulkan: "unknown"
  property string actionStatus: ""
  property string actionError: ""
  property string actionKind: ""
  property bool actionCloseOnSuccess: true

  readonly property color accent: "#56d2ff"
  readonly property color accentSoft: "#1c3444"
  readonly property color surface: "#0d1822"
  readonly property color surfaceRaised: "#122230"
  readonly property color border: "#2b485c"
  readonly property color success: "#71e7ad"
  readonly property color warning: "#ffd276"
  readonly property color danger: "#ff8fa1"
  readonly property color muted: "#a8bbc9"
  readonly property string uiFont: "sans-serif"

  readonly property string controlScript: decodeURIComponent(
    Qt.resolvedUrl("citizenctl").toString().replace(/^file:\/\//, "")
  )

  readonly property bool canLaunch: health === "ready"

  function open() { root.controller.show() }
  function close() { root.controller.hide() }

  function switchPanel(direction) {
    if (root.bar && typeof root.bar.switchPanelFrom === "function")
      return root.bar.switchPanelFrom(root.hostWidget || root, direction)
    return false
  }

  function runAction(action, closePanel) {
    if (actionProc.running) return

    root.actionKind = String(action || "")
    root.actionCloseOnSuccess = closePanel !== false
    root.actionError = ""
    root.actionStatus = "Aktion wird gestartet…"

    actionProc.command = ["bash", root.controlScript, root.actionKind]
    actionProc.running = true
  }

  function refreshStatus() {
    if (!statusProc.running) statusProc.running = true
  }

  function checkForUpdates() {
    if (updateProc.running) return
    root.checkingUpdate = true
    root.updateState = "checking"
    updateProc.running = true
  }

  function applyStatus(raw) {
    var values = {}
    var lines = String(raw || "").trim().split("\n")
    for (var i = 0; i < lines.length; i++) {
      var p = lines[i].indexOf("=")
      if (p <= 0) continue
      values[lines[i].slice(0, p)] = lines[i].slice(p + 1)
    }

    pluginVersion = values.plugin_version || "1.1.1"
    health = values.health || "setup"
    depsState = values.deps || "missing"
    depsMissing = values.deps_missing || ""
    helperState = values.helper || "missing"
    helperManaged = values.helper_managed || "missing"
    helperVersion = values.helper_version || ""
    updateState = values.update || "unknown"
    updateVersion = values.update_version || ""
    prefixState = values.prefix || "missing"
    prefixDetail = values.prefix_detail || ""
    launcherState = values.launcher || "missing"
    gameState = values.game || "missing"
    backendState = values.backend || "missing"
    backendVersion = values.backend_version || ""
    pluginManaged = values.plugin_managed || "manual"
    pluginAuto = values.plugin_auto || "false"
    pluginUpdate = values.plugin_update || "unavailable"
    pluginLocalCommit = values.plugin_local_commit || ""
    pluginRemoteCommit = values.plugin_remote_commit || ""
    autopilot = values.autopilot || "false"
    managedLUGVersion = values.managed_lug_version || ""
    managedWineVersion = values.managed_wine_version || ""
    managedDXVKVersion = values.managed_dxvk_version || ""
    maintenanceResult = values.maintenance_result || ""
    hardwareState = values.hardware || "unknown"
    hardwareReason = values.hardware_reason || ""
    hardwareGPU = values.hardware_gpu || ""
    hardwareVulkan = values.hardware_vulkan || "unknown"
  }

  function stateIcon(state) {
    if (state === "ready" || state === "current") return "✓"
    if (state === "available") return "↑"
    if (state === "warning") return "!"
    if (state === "partial" || state === "unknown") return "…"
    if (state === "checking") return "·"
    return "!"
  }

  function stateColor(state) {
    if (state === "ready" || state === "current") return success
    if (state === "available" || state === "warning") return warning
    if (state === "partial" || state === "unknown" || state === "checking") return accent
    return danger
  }

  function pluginUpdateText() {
    if (root.pluginUpdate === "available") return "Update verfügbar"
    if (root.pluginUpdate === "current") return "aktuell"
    if (root.pluginUpdate === "blocked-dirty") return "lokale Änderungen · Update pausiert"
    if (root.pluginUpdate === "blocked-diverged") return "Git-Stand abweichend · Update pausiert"
    if (root.pluginUpdate === "blocked-remote") return "Updatequelle geändert · Update pausiert"
    if (root.pluginUpdate === "zip") return "ZIP-Installation"
    return root.pluginUpdate
  }

  function pluginUpdateState() {
    if (root.pluginUpdate === "available") return "available"
    if (root.pluginUpdate === "current") return "current"
    if (String(root.pluginUpdate).indexOf("blocked-") === 0) return "warning"
    return "unknown"
  }

  function healthColor() {
    if (health === "hardware-blocked") return danger
    if (health === "ready") return success
    if (health === "recover-prefix" || health === "repair") return warning
    if (health === "checking") return accent
    return accent
  }

  function headline() {
    if (health === "hardware-blocked") return "HARDWARE NICHT SPIELBEREIT"
    if (health === "ready" && root.autopilot === "true") return "FLIGHT READY · AUTOPILOT"
    if (health === "ready") return "FLIGHT READY"
    if (health === "recover-prefix") return "SETUP REPAIR"
    if (health === "repair") return "REPAIR AVAILABLE"
    if (health === "checking") return "SYSTEM CHECK"
    return "READY FOR SETUP"
  }

  function subline() {
    if (health === "hardware-blocked")
      return root.hardwareReason !== "" ? root.hardwareReason : "Eine für Star Citizen geeignete Vulkan-GPU wurde nicht erkannt."
    if (health === "ready")
      return "Wine, DXVK und RSI Launcher sind geprüft. Star Citizen ist einen Klick entfernt."
    if (health === "install-game")
      return "Der Software-Stack ist bereit. Melde dich im offiziellen RSI Launcher an und starte dort den Spieldownload."
    if (health === "repair" || health === "launcher-repair")
      return "Omarchy Citizen kann den gesamten verwalteten Gaming-Stack automatisch reparieren."
    return "Omarchy Citizen wählt selbst einen kompatiblen Wine-Runner, erstellt den Prefix, installiert DXVK/Komponenten und den offiziellen RSI Launcher."
  }

  function primaryText() {
    if (health === "hardware-blocked") return "ⓘ  HARDWARE-DETAILS"
    if (health === "ready") return "▶  STAR CITIZEN STARTEN"
    if (health === "install-game") return "▶  RSI LAUNCHER ÖFFNEN"
    if (health === "repair" || health === "launcher-repair") return "↻  AUTOMATISCH REPARIEREN"
    return "▶  EINRICHTEN & STARTKLAR MACHEN"
  }

  function updateText() {
    if (checkingUpdate || updateState === "checking") return "wird geprüft"
    if (helperManaged === "external") return "extern"
    if (helperState !== "ready") return "nach Setup"
    if (updateState === "available")
      return updateVersion !== "" ? "Update " + updateVersion : "Update verfügbar"
    if (updateState === "current") return "aktuell"
    return "noch nicht geprüft"
  }

  onOpenedChanged: if (opened) refreshStatus()

  Process {
    id: statusProc
    command: ["bash", root.controlScript, "status"]
    stdout: StdioCollector {
      waitForEnd: true
      onStreamFinished: root.applyStatus(text)
    }
  }

  Process {
    id: updateProc
    command: ["bash", root.controlScript, "check-update"]
    stdout: StdioCollector {
      waitForEnd: true
      onStreamFinished: {
        root.checkingUpdate = false
        root.refreshStatus()
      }
    }
  }

  Process {
    id: actionProc
    running: false
    command: []

    stdout: StdioCollector {
      id: actionOut
      waitForEnd: true
    }

    stderr: StdioCollector {
      id: actionErr
      waitForEnd: true
    }

    onStarted: {
      if (root.actionKind === "primary" && root.health === "recover-prefix")
        root.actionStatus = "Reparatur-Fenster wird geöffnet…"
      else if (root.actionKind === "primary")
        root.actionStatus = "Setup/Start wird vorbereitet…"
      else
        root.actionStatus = "Aktion wird ausgeführt…"
    }

    onExited: function(exitCode) {
      if (exitCode === 0) {
        root.actionError = ""
        root.actionStatus = root.actionKind === "primary" && root.health === "install-game"
          ? "RSI Launcher wurde gestartet."
          : (root.actionKind === "primary" && root.health !== "ready"
              ? "Setup-Fenster wurde geöffnet. Omarchy Citizen übernimmt die technischen Schritte."
              : "Aktion wurde gestartet.")
        root.refreshStatus()

        if (root.actionCloseOnSuccess)
          closeAfterAction.restart()
      } else {
        var detail = String(actionErr.text || "").trim()
        root.actionStatus = ""
        root.actionError = detail !== ""
          ? detail
          : "Die Aktion konnte nicht gestartet werden. Bitte erstelle ein Support-Paket."
      }
    }
  }

  Timer {
    id: closeAfterAction
    interval: 350
    repeat: false
    onTriggered: root.close()
  }

  Timer {
    interval: 10000
    repeat: true
    running: root.opened
    onTriggered: root.refreshStatus()
  }

  KeyboardPanel {
    id: panel
    anchorItem: root.anchorItem
    owner: root.hostWidget || root
    bar: root.bar
    open: root.opened
    focusTarget: keyCatcher
    contentWidth: panel.fittedContentWidth(Style.space(450))
    contentHeight: panel.fittedContentHeight(content.implicitHeight)

    PanelKeyCatcher {
      id: keyCatcher
      anchors.fill: parent
      onCloseRequested: root.close()
      onTabRequested: function(direction) { root.switchPanel(direction) }

      Column {
        id: content
        width: parent.width
        spacing: Style.space(10)

        Rectangle {
          width: parent.width
          radius: 18
          color: root.surfaceRaised
          border.color: root.healthColor()
          border.width: 1
          implicitHeight: heroContent.implicitHeight + Style.space(24)

          Column {
            id: heroContent
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Style.space(16)
            spacing: Style.space(10)

            Row {
              width: parent.width
              spacing: Style.space(10)

              Rectangle {
                width: Style.space(34)
                height: Style.space(34)
                radius: 10
                color: root.accentSoft
                border.color: root.healthColor()
                border.width: 1

                Text {
                  anchors.centerIn: parent
                  text: "✦"
                  color: root.healthColor()
                  font.family: root.uiFont
                  font.pixelSize: Style.font.title
                  font.bold: true
                }
              }

              Column {
                spacing: Style.space(2)

                Text {
                  text: "CITIZEN LAUNCHER"
                  color: root.barForeground
                  font.family: root.uiFont
                  font.pixelSize: Style.font.title
                  font.bold: true
                }

                Text {
                  text: root.headline()
                  color: root.healthColor()
                  font.family: root.uiFont
                  font.pixelSize: Style.font.bodySmall
                  font.bold: true
                }
              }
            }

            Text {
              width: parent.width
              text: root.subline()
              color: root.muted
              font.family: root.uiFont
              font.pixelSize: Style.font.bodySmall
              wrapMode: Text.WordWrap
            }

            Button {
              width: parent.width
              text: actionProc.running
                ? "…  WIRD GESTARTET"
                : root.primaryText()
              foreground: root.barForeground
              fontFamily: root.uiFont
              bordered: true
              active: root.health === "ready" || actionProc.running
              onClicked: root.runAction(
                root.health === "hardware-blocked" ? "hardware-info" :
                  ((root.health === "repair" || root.health === "launcher-repair") ? "repair" : "primary"),
                root.health === "ready"
              )
            }

            Text {
              visible: root.actionStatus !== ""
              width: parent.width
              text: root.actionStatus
              color: root.accent
              font.family: root.uiFont
              font.pixelSize: Style.font.caption
              wrapMode: Text.WordWrap
            }

            Rectangle {
              visible: root.actionError !== ""
              width: parent.width
              radius: 10
              color: Qt.rgba(root.danger.r, root.danger.g, root.danger.b, 0.10)
              border.color: root.danger
              border.width: 1
              implicitHeight: actionErrorText.implicitHeight + Style.space(16)

              Text {
                id: actionErrorText
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.margins: Style.space(8)
                text: root.actionError
                color: root.danger
                font.family: root.uiFont
                font.pixelSize: Style.font.caption
                wrapMode: Text.WordWrap
              }
            }
          }
        }

        Rectangle {
          width: parent.width
          radius: 16
          color: root.surface
          border.color: root.border
          border.width: 1
          implicitHeight: simpleActions.implicitHeight + Style.space(20)

          Column {
            id: simpleActions
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Style.space(14)
            spacing: Style.space(9)

            Text {
              text: "HILFE OHNE LINUX-WISSEN"
              color: root.accent
              font.family: root.uiFont
              font.pixelSize: Style.font.bodySmall
              font.bold: true
            }

            Grid {
              id: helpGrid
              width: parent.width
              columns: 2
              columnSpacing: Style.space(7)
              rowSpacing: Style.space(7)
              readonly property real cellWidth: (width - columnSpacing) / 2

              Button {
                width: helpGrid.cellWidth
                text: "↻  Problem beheben"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("repair")
              }

              Button {
                width: helpGrid.cellWidth
                text: "▣  Support-Paket erstellen"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("support")
              }
            }

            Text {
              width: parent.width
              text: "Das Support-Paket sammelt Diagnoseinformationen automatisch und maskiert typische persönliche Daten vor dem Export."
              color: root.muted
              font.family: root.uiFont
              font.pixelSize: Style.font.caption
              wrapMode: Text.WordWrap
            }
          }
        }

        Rectangle {
          width: parent.width
          radius: 16
          color: root.surface
          border.color: root.border
          border.width: 1
          implicitHeight: quickStatus.implicitHeight + Style.space(20)

          Column {
            id: quickStatus
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Style.space(14)
            spacing: Style.space(8)

            Row {
              width: parent.width
              spacing: Style.space(8)

              StatusChip {
                label: "LUG"
                value: root.helperState === "ready" ? "bereit" : "fehlt"
                chipColor: root.stateColor(root.helperState)
              }

              StatusChip {
                label: "Launcher"
                value: root.launcherState === "ready" ? "bereit" : "Setup"
                chipColor: root.stateColor(root.launcherState)
              }

              StatusChip {
                label: "Game"
                value: root.gameState === "ready" ? "LIVE" : "Setup"
                chipColor: root.stateColor(root.gameState)
              }
            }

            Text {
              width: parent.width
              text: "Version " + root.pluginVersion +
                " · Autopilot " + (root.autopilot === "true" ? "AKTIV" : "aus") +
                " · LUG " + (root.managedLUGVersion !== "" ? root.managedLUGVersion : "—") +
                " · Wine " + (root.managedWineVersion !== "" ? root.managedWineVersion : "—")
              color: root.muted
              font.family: root.uiFont
              font.pixelSize: Style.font.caption
              wrapMode: Text.WordWrap
            }
          }
        }

        Button {
          width: parent.width
          text: root.advancedVisible ? "▲  Erweiterte Optionen ausblenden" : "▼  Erweiterte Optionen anzeigen"
          foreground: root.barForeground
          fontFamily: root.uiFont
          bordered: true
          onClicked: root.advancedVisible = !root.advancedVisible
        }

        Rectangle {
          visible: root.advancedVisible
          width: parent.width
          radius: 16
          color: root.surface
          border.color: root.border
          border.width: 1
          implicitHeight: visible ? advancedContent.implicitHeight + Style.space(20) : 0

          Column {
            id: advancedContent
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Style.space(14)
            spacing: Style.space(10)

            Text {
              text: "ERWEITERTE OPTIONEN"
              color: root.accent
              font.family: root.uiFont
              font.pixelSize: Style.font.bodySmall
              font.bold: true
            }

            StatusLine {
              label: "GPU / Vulkan"
              state: root.hardwareState === "blocked" ? "warning" : (root.hardwareState === "ready" ? "ready" : "unknown")
              value: root.hardwareState === "blocked" ? "nicht spielbereit" : (root.hardwareGPU !== "" ? root.hardwareGPU : root.hardwareVulkan)
            }

            StatusLine {
              label: "Abhängigkeiten"
              state: root.depsState
              value: root.depsState === "ready" ? "bereit" : root.depsMissing
            }

            StatusLine {
              label: "Support-Helper"
              state: root.helperState
              value: root.helperState === "ready"
                ? ((root.helperVersion !== "" ? root.helperVersion : "gefunden") +
                   " · optional")
                : "nicht installiert"
            }

            StatusLine {
              label: "Wine-Prefix"
              state: root.prefixState
              value: root.prefixState === "ready"
                ? "initialisiert"
                : (root.prefixState === "partial" ? "unvollständig · Reparatur möglich" : "nicht eingerichtet")
            }

            StatusLine {
              label: "RSI Launcher"
              state: root.launcherState
              value: root.launcherState === "ready" ? "installiert" : "fehlt"
            }

            PanelSeparator { foreground: root.barForeground }

            StatusLine {
              label: "Autopilot"
              state: root.autopilot === "true" ? "ready" : "warning"
              value: root.autopilot === "true"
                ? "aktiv · Wartung alle 6 Stunden"
                : "nicht aktiviert"
            }

            StatusLine {
              label: "Managed Wine"
              state: root.managedWineVersion !== "" ? "ready" : "unknown"
              value: root.managedWineVersion !== "" ? root.managedWineVersion : "wird beim Setup geladen"
            }

            StatusLine {
              label: "Managed DXVK"
              state: root.managedDXVKVersion !== "" ? "ready" : "unknown"
              value: root.managedDXVKVersion !== "" ? root.managedDXVKVersion : "nach Prefix-Setup"
            }

            Text {
              text: "PLUGIN-UPDATES"
              color: root.accent
              font.family: root.uiFont
              font.pixelSize: Style.font.bodySmall
              font.bold: true
            }

            StatusLine {
              label: "Update-Modus"
              state: root.backendState === "ready" ? "ready" : "missing"
              value: root.pluginManaged === "git"
                ? (root.pluginAuto === "true" ? "automatisch · alle ~6h" : "Git · manuell")
                : "ZIP · Git-Installation für Auto-Updates nötig"
            }

            StatusLine {
              label: "Plugin-Stand"
              state: root.pluginUpdateState()
              value: root.pluginUpdateText()
            }

            Grid {
              id: pluginUpdateGrid
              width: parent.width
              columns: 2
              columnSpacing: Style.space(7)
              rowSpacing: Style.space(7)
              readonly property real cellWidth: (width - columnSpacing) / 2

              Button {
                width: pluginUpdateGrid.cellWidth
                text: "Plugin-Update prüfen"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("plugin-update-check")
              }

              Button {
                width: pluginUpdateGrid.cellWidth
                text: "Plugin jetzt aktualisieren"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                active: root.pluginUpdate === "available"
                onClicked: root.runAction("plugin-update-now")
              }

              Button {
                width: pluginUpdateGrid.cellWidth
                text: root.pluginAuto === "true" ? "Auto-Updates ausschalten" : "Auto-Updates einschalten"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction(root.pluginAuto === "true" ? "plugin-auto-disable" : "plugin-auto-enable")
              }

              Button {
                width: pluginUpdateGrid.cellWidth
                text: "Updater-Log öffnen"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("updater-log")
              }
            }

            Text {
              width: parent.width
              text: root.pluginManaged === "git"
                ? "Auto-Updates verwenden Omarchys eigenen Plugin-Updater mit Validierung und Rollback. Das beim Aktivieren vorhandene Git-Remote wird als vertrauenswürdige Quelle gespeichert."
                : "Die aktuelle ZIP-Installation bleibt bewusst ohne unbekannte Update-Quelle. Sobald Omarchy Citizen aus einem Git-Repository installiert wird, erkennt das Backend dessen origin automatisch."
              color: root.muted
              font.family: root.uiFont
              font.pixelSize: Style.font.caption
              wrapMode: Text.WordWrap
            }

            PanelSeparator { foreground: root.barForeground }

            Grid {
              id: setupGrid
              width: parent.width
              columns: 2
              columnSpacing: Style.space(7)
              rowSpacing: Style.space(7)
              readonly property real cellWidth: (width - columnSpacing) / 2

              Button {
                width: setupGrid.cellWidth
                text: "Autopilot synchronisieren"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("autopilot-maintain")
              }

              Button {
                width: setupGrid.cellWidth
                text: "Gaming-Stack aktualisieren"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("autopilot-maintain")
              }

              Button {
                width: setupGrid.cellWidth
                text: "Omarchy aktualisieren"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("system-update")
              }

              Button {
                width: setupGrid.cellWidth
                text: "Support-Helper öffnen"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("helper")
              }

              Button {
                width: setupGrid.cellWidth
                text: "Wine-Selbsttest"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("wine-doctor")
              }

              Button {
                width: setupGrid.cellWidth
                text: "DXVK automatisch prüfen"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("autopilot-maintain")
              }

              Button {
                width: setupGrid.cellWidth
                text: "Gaming-Stack reparieren"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("repair")
              }

              Button {
                width: setupGrid.cellWidth
                text: "Launcher-Dateien reparieren"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("repair")
              }

              Button {
                width: setupGrid.cellWidth
                text: "Start-Log öffnen"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("log")
              }

              Button {
                width: setupGrid.cellWidth
                text: "Plugin-Debuglog öffnen"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("debug-log")
              }
            }

            PanelSeparator { foreground: root.barForeground }

            Text {
              text: "NGL · EXPERIMENTELL"
              color: root.warning
              font.family: root.uiFont
              font.pixelSize: Style.font.bodySmall
              font.bold: true
            }

            Grid {
              id: nglGrid
              width: parent.width
              columns: 2
              columnSpacing: Style.space(7)
              rowSpacing: Style.space(7)
              readonly property real cellWidth: (width - columnSpacing) / 2

              Button {
                width: nglGrid.cellWidth
                text: "NGL Download"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("ngl-page")
              }

              Button {
                width: nglGrid.cellWidth
                text: "NGL installieren"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("ngl-install")
              }
            }

            Text {
              width: parent.width
              text: "NGL bleibt bewusst optional. Der normale RSI/LUG-Weg funktioniert unabhängig davon."
              color: root.warning
              opacity: 0.82
              font.family: root.uiFont
              font.pixelSize: Style.font.caption
              wrapMode: Text.WordWrap
            }
          }
        }

        Text {
          width: parent.width
          text: "Ziel: spielen statt Linux administrieren. Für Support genügt normalerweise „Support-Paket erstellen“."
          color: root.muted
          font.family: root.uiFont
          font.pixelSize: Style.font.caption
          wrapMode: Text.WordWrap
        }
      }
    }
  }

  component StatusChip: Rectangle {
    id: chip
    property string label: ""
    property string value: ""
    property color chipColor: root.accent

    radius: 999
    color: Qt.rgba(chip.chipColor.r, chip.chipColor.g, chip.chipColor.b, 0.12)
    border.color: chip.chipColor
    border.width: 1
    implicitWidth: chipRow.implicitWidth + Style.space(16)
    implicitHeight: chipRow.implicitHeight + Style.space(9)

    Row {
      id: chipRow
      anchors.centerIn: parent
      spacing: Style.space(5)

      Text {
        text: chip.label
        color: root.muted
        font.family: root.uiFont
        font.pixelSize: Style.font.caption
      }

      Text {
        text: chip.value
        color: chip.chipColor
        font.family: root.uiFont
        font.pixelSize: Style.font.caption
        font.bold: true
      }
    }
  }

  component StatusLine: Rectangle {
    id: statusLine
    property string label: ""
    property string value: ""
    property string state: "unknown"

    width: parent.width
    radius: 11
    color: root.surfaceRaised
    border.color: root.border
    border.width: 1
    implicitHeight: lineRow.implicitHeight + Style.space(12)

    Row {
      id: lineRow
      anchors.left: parent.left
      anchors.right: parent.right
      anchors.verticalCenter: parent.verticalCenter
      anchors.margins: Style.space(10)
      spacing: Style.space(8)

      Rectangle {
        width: 9
        height: 9
        radius: 5
        color: root.stateColor(statusLine.state)
      }

      Text {
        width: Style.space(118)
        text: statusLine.label
        color: root.muted
        font.family: root.uiFont
        font.pixelSize: Style.font.bodySmall
      }

      Text {
        width: Math.max(0, lineRow.width - Style.space(158))
        text: root.stateIcon(statusLine.state) + " " + statusLine.value
        color: root.barForeground
        font.family: root.uiFont
        font.pixelSize: Style.font.bodySmall
        elide: Text.ElideRight
      }
    }
  }
}
