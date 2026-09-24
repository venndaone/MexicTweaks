<# : Batch-Teil (startet den PowerShell-Teil weiter unten)
@echo off
set "SELF=%~f0"
net session >nul 2>&1
if errorlevel 1 (
    powershell -NoProfile -Command "Start-Process -FilePath $env:SELF -Verb RunAs"
    exit /b
)
powershell -NoProfile -ExecutionPolicy Bypass -STA -Command "iex (Get-Content -LiteralPath $env:SELF -Raw -Encoding UTF8)"
exit /b
#>

# ================= Registry Backup Tool =================
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[Windows.Forms.Application]::EnableVisualStyles()

function Log($msg) {
    $log.AppendText("$msg`r`n")
    [Windows.Forms.Application]::DoEvents()
}

$form = New-Object Windows.Forms.Form
$form.Text = 'Registry Backup'
$form.ClientSize = New-Object Drawing.Size(550, 385)
$form.StartPosition = 'CenterScreen'
$form.FormBorderStyle = 'FixedDialog'
$form.MaximizeBox = $false
$form.Font = New-Object Drawing.Font('Segoe UI', 9)

$lbl = New-Object Windows.Forms.Label
$lbl.Text = 'Zielordner für das Backup:'
$lbl.Location = New-Object Drawing.Point(15, 15)
$lbl.AutoSize = $true
$form.Controls.Add($lbl)

$txt = New-Object Windows.Forms.TextBox
$txt.Location = New-Object Drawing.Point(15, 38)
$txt.Size = New-Object Drawing.Size(420, 24)
$txt.Text = Join-Path ([Environment]::GetFolderPath('MyDocuments')) 'Registry-Backups'
$form.Controls.Add($txt)

$btnBrowse = New-Object Windows.Forms.Button
$btnBrowse.Text = 'Durchsuchen...'
$btnBrowse.Location = New-Object Drawing.Point(445, 36)
$btnBrowse.Size = New-Object Drawing.Size(90, 27)
$btnBrowse.Add_Click({
    $dlg = New-Object Windows.Forms.FolderBrowserDialog
    $dlg.Description = 'Wähle den Ordner, in dem das Backup gespeichert werden soll'
    $dlg.ShowNewFolderButton = $true
    if (Test-Path $txt.Text) { $dlg.SelectedPath = $txt.Text }
    if ($dlg.ShowDialog() -eq 'OK') { $txt.Text = $dlg.SelectedPath }
})
$form.Controls.Add($btnBrowse)

$chk = New-Object Windows.Forms.CheckBox
$chk.Text = 'Zusätzlich Windows-Wiederherstellungspunkt erstellen (empfohlen)'
$chk.Location = New-Object Drawing.Point(15, 72)
$chk.Size = New-Object Drawing.Size(520, 22)
$chk.Checked = $true
$form.Controls.Add($chk)

$btnStart = New-Object Windows.Forms.Button
$btnStart.Text = 'Backup starten'
$btnStart.Location = New-Object Drawing.Point(15, 102)
$btnStart.Size = New-Object Drawing.Size(520, 36)
$btnStart.Font = New-Object Drawing.Font('Segoe UI', 10, [Drawing.FontStyle]::Bold)
$form.Controls.Add($btnStart)

$log = New-Object Windows.Forms.TextBox
$log.Multiline = $true
$log.ReadOnly = $true
$log.ScrollBars = 'Vertical'
$log.Location = New-Object Drawing.Point(15, 150)
$log.Size = New-Object Drawing.Size(520, 220)
$log.Font = New-Object Drawing.Font('Consolas', 9)
$form.Controls.Add($log)

$btnStart.Add_Click({
    $target = $txt.Text.Trim()
    if (-not $target) {
        [Windows.Forms.MessageBox]::Show('Bitte zuerst einen Zielordner wählen.', 'Registry Backup') | Out-Null
        return
    }

    $stamp = Get-Date -Format 'yyyy-MM-dd_HH-mm-ss'
    $dir = Join-Path $target "Registry-Backup_$stamp"
    try {
        New-Item -ItemType Directory -Path $dir -Force -ErrorAction Stop | Out-Null
    } catch {
        [Windows.Forms.MessageBox]::Show("Ordner konnte nicht erstellt werden:`n$($_.Exception.Message)", 'Fehler') | Out-Null
        return
    }

    $btnStart.Enabled = $false; $btnBrowse.Enabled = $false; $chk.Enabled = $false; $txt.Enabled = $false
    $log.Clear()
    Log "Backup-Ordner: $dir"
    Log ''

    # --- Wiederherstellungspunkt ---
    if ($chk.Checked) {
        Log 'Erstelle Wiederherstellungspunkt (kann bis zu 1 Min. dauern)...'
        $job = Start-Job -ArgumentList $stamp -ScriptBlock {
            param($s)
            try {
                Enable-ComputerRestore -Drive "$env:SystemDrive\" -ErrorAction SilentlyContinue
                $w = $null
                Checkpoint-Computer -Description "Vor Registry-Tweaks $s" -RestorePointType MODIFY_SETTINGS `
                    -ErrorAction Stop -WarningVariable w -WarningAction SilentlyContinue
                if ($w) { "WARN: $w" } else { 'OK' }
            } catch { "ERR: $($_.Exception.Message)" }
        }
        while ($job.State -eq 'Running') { [Windows.Forms.Application]::DoEvents(); Start-Sleep -Milliseconds 150 }
        $res = (Receive-Job $job) -join ' '
        Remove-Job $job -Force
        if ($res -eq 'OK') { Log '  -> Wiederherstellungspunkt erstellt.' }
        elseif ($res -like 'WARN:*') { Log "  -> Hinweis: $($res.Substring(6))"; Log '     (Windows erlaubt standardmäßig nur einen Punkt pro 24 Std.)' }
        else { Log "  -> Fehlgeschlagen: $($res -replace '^ERR: ','')" }
        Log ''
    }

    # --- Registry-Export (entspricht "Computer -> Exportieren") ---
    $hives = [ordered]@{
        'HKEY_LOCAL_MACHINE'  = 'HKLM'
        'HKEY_CURRENT_USER'   = 'HKCU'
        'HKEY_CLASSES_ROOT'   = 'HKCR'
        'HKEY_USERS'          = 'HKU'
        'HKEY_CURRENT_CONFIG' = 'HKCC'
    }
    $ok = 0
    foreach ($h in $hives.GetEnumerator()) {
        $file = Join-Path $dir "$($h.Key).reg"
        Log "Exportiere $($h.Key) ..."
        $p = Start-Process -FilePath reg.exe -ArgumentList @('export', $h.Value, "`"$file`"", '/y') `
             -WindowStyle Hidden -PassThru
        $null = $p.Handle
        while (-not $p.HasExited) { [Windows.Forms.Application]::DoEvents(); Start-Sleep -Milliseconds 100 }
        if ($p.ExitCode -eq 0 -and (Test-Path $file)) {
            $mb = [math]::Round((Get-Item $file).Length / 1MB, 1)
            Log "  -> OK ($mb MB)"
            $ok++
        } else {
            Log "  -> FEHLER (Exitcode $($p.ExitCode))"
        }
    }

    $info = @"
Registry-Backup vom $(Get-Date -Format 'dd.MM.yyyy HH:mm:ss')
Computer: $env:COMPUTERNAME   Benutzer: $env:USERNAME

WIEDERHERSTELLEN:
Variante 1 (empfohlen): Wiederherstellungspunkt nutzen
  Win + R -> rstrui -> Punkt "Vor Registry-Tweaks ..." auswählen.

Variante 2: .reg-Dateien importieren
  Die gewünschte .reg-Datei doppelklicken -> Ja -> PC neu starten.
  Für die Gaming-Tweaks reichen meist HKEY_LOCAL_MACHINE.reg und HKEY_CURRENT_USER.reg.
  Hinweis: Einzelne Schlüssel, die gerade von Windows benutzt werden, lassen sich
  evtl. nicht importieren. Das ist normal.
"@
    Set-Content -Path (Join-Path $dir 'LIESMICH.txt') -Value $info -Encoding UTF8

    Log ''
    Log "Fertig: $ok von $($hives.Count) Bereichen gesichert."
    $btnStart.Enabled = $true; $btnBrowse.Enabled = $true; $chk.Enabled = $true; $txt.Enabled = $true

    [Windows.Forms.MessageBox]::Show("Backup abgeschlossen ($ok/$($hives.Count)).`n`n$dir", 'Registry Backup') | Out-Null
    Start-Process explorer.exe -ArgumentList "`"$dir`""
})

$form.ShowDialog() | Out-Null
