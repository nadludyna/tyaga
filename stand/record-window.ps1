# Types links into ТЯГА's window (see run-window.sh), starts them and records the whole screen
# to <name>.mp4 in the temp folder. The window is clicked where its links box is at 125 %.
param([string]$links = "", [int]$wait = 0, [int]$secs = 10, [string]$name = "rec", [int]$fps = 30)
Add-Type -AssemblyName System.Windows.Forms
Add-Type @"
using System; using System.Runtime.InteropServices;
public class MR { [DllImport("user32.dll")] public static extern bool SetCursorPos(int x,int y);
[DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
[DllImport("user32.dll")] public static extern void mouse_event(int f,int x,int y,int d,int e); }
"@
[void][MR]::SetProcessDPIAware()
$S = $env:TEMP
$rec = Start-Process ffmpeg -ArgumentList "-v error -y -f gdigrab -framerate $fps -t $secs -i desktop -c:v libx264 -preset ultrafast -crf 12 `"$S\$name.mp4`"" -PassThru -WindowStyle Hidden
Start-Sleep -Milliseconds 700
if ($links) {
  [void][MR]::SetCursorPos(860,490); [MR]::mouse_event(2,0,0,0,0); [MR]::mouse_event(4,0,0,0,0)
  Start-Sleep -Milliseconds 300
  foreach ($l in $links.Split(",")) { [System.Windows.Forms.SendKeys]::SendWait($l); [System.Windows.Forms.SendKeys]::SendWait("+{ENTER}") }
  [System.Windows.Forms.SendKeys]::SendWait("{ENTER}")
  [void][MR]::SetCursorPos(1880,60)
}
$rec.WaitForExit()
"recorded $name"
