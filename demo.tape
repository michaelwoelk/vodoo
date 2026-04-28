Output screenshots/demo.gif

Set Shell "bash"
Set Width 1200
Set Height 700
Set FontSize 14
Set Theme "Dracula"
Set Padding 20

# Build and start (hidden, GIF starts with app already running)
Hide
Type "./vodoo --data dataTest"
Enter
Sleep 3s
Show
Sleep 4s
Screenshot screenshots/01_dashboard.png

# Jahr
Right
Sleep 4s
Screenshot screenshots/02_jahr.png

# Monate
Right
Sleep 4s
Screenshot screenshots/03_monate.png

# Navigate months
Up
Sleep 1s
Up
Sleep 1s
Down
Sleep 4s
Screenshot screenshots/04_monate_april.png

# Feiertage & Abwesenheiten
Right
Sleep 4s
Screenshot screenshots/05_feiertage_abwesenheiten.png
