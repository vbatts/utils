
email.xpm.go: email.xpm
	go-bindata -o $@ $<

email.xpm: email.png
	convert $< $@

email.png:
	curl -o $@ http://icons.iconarchive.com/icons/zerode/plump/256/Mail-icon.png

