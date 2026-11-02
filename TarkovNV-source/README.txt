=====================================================================
  TARKOV NV  v1.7.1  -  Visual preset controller for Escape From Tarkov
=====================================================================

A modern replacement for using RivaTuner's colour tab with Tarkov.
It does the same job (it writes the display's gamma ramp: brightness,
contrast and gamma per R/G/B channel). RivaTuner is NOT needed, and
you don't have to pick a "primary display adapter" any more.

Works on any GPU (NVIDIA, AMD, Intel). NVIDIA Digital Vibrance is
an extra and is greyed out automatically on non-NVIDIA cards.

Unofficial fan-made tool. Not affiliated with Battlestate Games.


IF WINDOWS DEFENDER BLOCKS IT
-----------------------------
TarkovNV is a small, unsigned program, and antivirus scanners
sometimes wrongly flag those (a "false positive"). The full source
code is published alongside it. If it gets blocked:
  1. Open Windows Security > Virus & threat protection >
     Protection history, and note the detection name.
  2. Report it as a false positive at
     https://www.microsoft.com/wdsi/filesubmission
  3. To run it meanwhile: in Protection history choose
     Actions > Allow on device. Only do this for a copy you got
     from the official download.


QUICK START
-----------
1. Put TarkovNV.exe anywhere (e.g. Documents\TarkovNV) and run it.
   No installer, nothing else to install.
   - Windows SmartScreen may say "Windows protected your PC" because
     the exe isn't code-signed: click "More info" -> "Run anyway".
2. TARGET DISPLAY (top left) auto-selects your primary monitor.
   Click it to pick another monitor if Tarkov runs on a different one.
3. Click a preset in the list. It's applied immediately ("LIVE").
4. Drag the sliders. Changes show on screen in real time and are
   saved automatically.
5. If you see the yellow "Windows caps how far gamma can be pushed"
   bar, click UNLOCK (ADMIN) once and restart your PC. This is the
   same GdiIcmGammaRange tweak RivaTuner users apply. Without it,
   strong night presets get rejected by Windows.


DEFAULT HOTKEYS (all can be changed)
------------------------------------
  Ctrl + Num 0 ........ DEFAULT (normal colours)
  Ctrl + Num 1 ........ NIGHT RAID
  Ctrl + Num 2 ........ DARK INTERIORS
  Ctrl + Num 3 ........ DAY CLARITY
  Ctrl + Num 4 ........ FOG / OVERCAST
  Ctrl + Num + / - .... next / previous preset
  Ctrl + Num * ........ toggle DEFAULT <-> your last preset

To change a hotkey: click the hotkey box, press the new combo.
Esc cancels, Backspace clears. A red "IN USE" tag means another
program already owns that combo, so pick a different one.
Num-pad hotkeys need NUM LOCK on.

The bundled presets are only starting points. Tune them for your
monitor.


CONTROLS
--------
  Sliders .......... drag; hold Shift for fine control; mouse wheel
                     = one step; double-click = reset that slider
  Exact values ..... click the number next to any slider, type a
                     value (e.g. 180) and press Enter
  Ranges ........... Brightness -250..+250, Contrast -100..+250,
                     Gamma 0.10..5.00, Vibrance 0..250%.
  LINKED RGB ....... edit all channels together
  RED/GREEN/BLUE ... edit a single channel (like RivaTuner)
  Response curve ... live graph of the colour curve
  Shadow detail .... before/after strip of the dark tones, so you
                     can see how much shadow detail you're lifting
  UNDO / REDO ...... buttons next to the tabs, or Ctrl+Z / Ctrl+Y.
                     Each finished change is one step (a whole
                     slider drag, a typed value, a rename, a deleted
                     preset, a hotkey or a setting). Up to 100 steps
                     are kept while TarkovNV is open.
  Tray icon ........ right-click to switch presets or exit


PER-MONITOR PRESETS (new in v1.7)
---------------------------------
Each monitor keeps its own preset list, active preset and hotkeys.
Pick a monitor under TARGET DISPLAY and its list loads. The first
time you pick a new monitor, it starts with a copy of your current
presets, so you only adjust what's different. The display menu
shows how many presets each monitor has.
Settings > Behaviour:
  - "Separate presets for each monitor" (on by default). Turn it off
    to share one list across all monitors.
  - "COPY THIS LIST TO ALL MONITORS" overwrites the other monitors'
    lists with the current one.
Monitors are recognised by their hardware ID, so their presets
follow them even if Windows renumbers DISPLAY1/DISPLAY2.


RGB & TONE TAB (new in v1.5)
----------------------------
01 PER-CHANNEL: Brightness, Contrast and Gamma for RED, GREEN and
   BLUE side by side, so you can tune each channel for your
   monitor. Editing a column unlinks the channels. "LINK ALL TO
   GREEN" joins them again, and "RESET RGB" makes them neutral.
02 TONE & COLOUR BALANCE (all channels):
   SHADOWS ...... lifts only the dark areas (players in corners,
                  under trees, inside buildings) without washing
                  out the rest. True black stays black.
   HIGHLIGHTS ... negative values tame glare from sun, snow,
                  flashlights and bright windows.
   TEMPERATURE .. cooler (bluer) <-> warmer. Night maps often read
                  better slightly warm.
   TINT ......... greener <-> more magenta.
03 COLOUR CHECK: typical Tarkov colours (shadows, foliage, gear,
   skin, sky) shown at default and with your preset.

TEMPLATES (Editor tab): one-click starting points that load into the
selected preset and keep its name and hotkey: Night Raid, Pitch
Black, Dark Interiors, Labs / Bright Lights, Day Clarity, Fog,
Snow / Sun Glare, Warm Night, NVG Green, Neutral. Ctrl+Z undoes.

LEVELS (Test Scenes tab): a calibration screen for your monitor.
Raise SHADOWS or GAMMA until square 1 of the black level row is
barely visible. Lower HIGHLIGHTS if the bright squares disappear
into white. A SHADOWS quick slider is also under every test scene.


DIGITAL VIBRANCE UP TO 250%
---------------------------
  NVIDIA:   0-100%  = NVIDIA's own Digital Vibrance (same as NVIDIA
                      Control Panel; 50% = normal, 100% = driver max)
            101-250% = driver at max PLUS a TarkovNV saturation boost
  AMD / Intel: the whole range uses the boost (50% = normal).
The badge next to the slider says which is active.

The boost is a Windows full-screen colour filter (the same system
Windows' own colour filters use, Windows 8 or newer). Things to know:
  - It applies to ALL monitors, not just the target display.
  - Use BORDERLESS WINDOWED in Tarkov. In exclusive fullscreen the
    boost may not show. NVIDIA's 0-100% part always works.
  - It switches off automatically when TarkovNV closes, and while
    you hold HOLD - TRUE DEFAULT.
  - It can't run alongside Windows Settings > Accessibility >
    Color filters, or the Windows Magnifier. Turn those off. If
    the badge says "BOOST BLOCKED BY WINDOWS", that's the reason.


TEST SCENES
-----------
Open the TEST SCENES tab to judge a preset without loading a raid.
  - Built-in shots: BUNKER + BUNKER II (dark), ALLEY (normal),
    OUTDOOR (bright).
  - The picture shows exactly how the game looks with the active
    preset (your monitor applies the preset to everything, including
    this window).
  - HOLD - TRUE DEFAULT: while held, your whole screen switches to
    default colours, so you can compare. Let go to return.
  - The sliders under the picture edit the active preset live.
  - YOUR SCREENSHOTS: picks up your Tarkov screenshots from
    Documents\Escape from Tarkov\Screenshots (PrtSc in-game), or
    drop any PNG/JPG into %APPDATA%\TarkovNV\scenes.


TARKOV TIPS
-----------
- Use BORDERLESS WINDOWED in Tarkov for the most reliable results.
  Hotkeys and the on-screen indicator then work in-game.
- "Keep colours applied" (Settings, on by default) re-applies your
  preset if the game, an alt-tab or the driver resets it.
- Auto-switch (Settings > Escape From Tarkov): picks a preset when
  EscapeFromTarkov.exe starts and goes back to your previous one
  when the game closes.
- TURN OFF HDR in Windows. Gamma ramps don't work with HDR on.
  Also turn off Windows 11 "Auto color management" if colours look
  wrong.
- If the on-screen indicator ever causes problems in exclusive
  fullscreen, turn it off in Settings.


ANTI-CHEAT NOTE
---------------
TarkovNV never touches or reads the game process. It only checks
the process list to see whether EscapeFromTarkov.exe is running.
It changes the display's colour table through standard Windows /
NVIDIA driver functions, exactly like RivaTuner, NVIDIA Control
Panel or vibranceGUI. That's widely used, but use at your own
discretion: Battlestate's policy is theirs to decide.


IF YOUR SCREEN IS STUCK TOO DARK / BRIGHT
-----------------------------------------
- Press Ctrl + Num 0 (DEFAULT), or
- Right-click the tray icon -> "Exit (restore colours)", or
- Restart the PC. Windows resets the gamma ramp on reboot.


SETTINGS & UNINSTALL
--------------------
Settings live in  %APPDATA%\TarkovNV\config.json
Portable mode: create an empty file named portable.txt next to
TarkovNV.exe and settings will be stored next to the exe instead.

TarkovNV never starts with Windows. It only runs when you open it
(v1.2 also removes any startup entry an older version added).

To uninstall: exit from the tray, delete TarkovNV.exe and the %APPDATA%\TarkovNV folder.
Optional: the gamma-range unlock is the DWORD value
GdiIcmGammaRange under
HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\ICM
(delete it to undo).


SUPPORT
-------
TarkovNV is free. If it helps you in raids, click SUPPORT at the top
right of the app (or in the tray menu). It opens the TarkovNV GitHub
page, where the Sponsor / Buy Me a Coffee buttons are:
  https://github.com/g-hostproxy/TarkovNV
Thank you!


CREDITS
-------
Fonts (embedded, SIL Open Font License 1.1, see /fonts in source):
Barlow Condensed, Saira Stencil One, Share Tech Mono.
Built with Go (pure Win32 + GDI/GDI+, no runtime required).

Build from source:  GOOS=windows GOARCH=amd64 go build
                    -ldflags "-H windowsgui -s -w" -o TarkovNV.exe .
