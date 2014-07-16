# slackware-sync

Really it is not limited to slackware, but that is what I was thinking when I
wrote the first version.

# install

Install this command

	go get github.com/vbatts/freezing-octo-hipster/cmd/slackware-sync

Copy the `slackware-sync.toml` to `~/.slackware-sync.toml`, and edit as needed.

Then either run it periodically, or add a crontab to run `slackware-sync -q`.

