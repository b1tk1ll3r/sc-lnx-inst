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

  property string pluginVersion: "0.6.4"
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

    pluginVersion = values.plugin_version || "0.6.4"
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
    if (health === "ready") return success
    if (health === "recover-prefix" || health === "repair") return warning
    if (health === "checking") return accent
    return accent
  }

  function headline() {
    if (health === "ready") return "FLIGHT READY"
    if (health === "recover-prefix") return "SETUP REPAIR"
    if (health === "repair") return "REPAIR AVAILABLE"
    if (health === "checking") return "SYSTEM CHECK"
    return "READY FOR SETUP"
  }

  function subline() {
    if (health === "ready")
      return "Alles Wesentliche wurde gefunden. Star Citizen ist einen Klick entfernt."
    if (health === "recover-prefix")
      return "Eine frühere Wine-Einrichtung ist nur teilweise vorhanden. Omarchy Citizen kann sie sichern und sauber neu aufsetzen."
    if (health === "repair")
      return "Omarchy Citizen hat eine reparierbare Lücke erkannt."
    if (health === "install-game")
      return "Die Linux-Basis steht. Der RSI Launcher führt dich durch die Spielinstallation."
    if (health === "install")
      return "Die Linux-Basis steht. Der grafische Star-Citizen-Assistent übernimmt den Rest."
    return "Omarchy Citizen richtet Abhängigkeiten, LUG Helper und Star-Citizen-Basis geführt ein."
  }

  function primaryText() {
    if (health === "ready") return "▶  STAR CITIZEN STARTEN"
    if (health === "recover-prefix") return "↻  SETUP SICHER REPARIEREN"
    if (health === "repair") return "↻  ERKANNTES PROBLEM BEHEBEN"
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
        root.actionStatus = root.actionKind === "primary" && root.health === "recover-prefix"
          ? "Reparatur-Fenster wurde geöffnet. Dort läuft die sichere Wiederherstellung."
          : (root.actionKind === "primary" && root.health !== "ready"
              ? "Setup-Fenster wurde geöffnet. Folge dort einfach den angezeigten Schritten."
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
                  text: "OMARCHY CITIZEN"
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
                root.health === "repair" ? "repair" : "primary",
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
              text: "Version " + root.pluginVersion + " · LUG " +
                (root.helperVersion !== "" ? root.helperVersion : "nicht eingerichtet") +
                " · Plugin: " + (root.pluginManaged === "git" ? (root.pluginAuto === "true" ? "Auto" : "Git") : "ZIP")
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
              label: "Abhängigkeiten"
              state: root.depsState
              value: root.depsState === "ready" ? "bereit" : root.depsMissing
            }

            StatusLine {
              label: "LUG Helper"
              state: root.helperState
              value: root.helperState === "ready"
                ? ((root.helperVersion !== "" ? root.helperVersion : "gefunden") +
                   (root.helperManaged === "aur" ? " · AUR" : " · extern"))
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
              label: "Start-Script"
              state: root.launcherState
              value: root.launcherState === "ready" ? "bereit" : "fehlt"
            }

            PanelSeparator { foreground: root.barForeground }

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
                text: root.checkingUpdate ? "Update wird geprüft…" : "LUG Update prüfen"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.checkForUpdates()
              }

              Button {
                width: setupGrid.cellWidth
                text: "LUG aktualisieren"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("update-helper")
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
                text: "LUG Helper öffnen"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("helper")
              }

              Button {
                width: setupGrid.cellWidth
                text: "Wine-Runner"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("runners")
              }

              Button {
                width: setupGrid.cellWidth
                text: "DXVK verwalten"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("dxvk")
              }

              Button {
                width: setupGrid.cellWidth
                text: "RSI reparieren"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("repair-rsi")
              }

              Button {
                width: setupGrid.cellWidth
                text: "Start-Script reparieren"
                foreground: root.barForeground
                fontFamily: root.uiFont
                bordered: true
                onClicked: root.runAction("repair-launch")
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
