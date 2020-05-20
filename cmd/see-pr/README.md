# see-pr

Add the github PR refspec to a local git repo.

## Install

	go get github.com/vbatts/utils/cmd/see-pr

## Usage

	$ see-pr -h
	Usage of see-pr:
	  -config string
	        path to the git config (default ".git/config")
	  -path string
	        local path of the git repo (default ".")
	  -remote string
	        upstream remote name (default "origin")

Then to let it update your `.git/config`:

	$ see-pr
	INFO[0000] reading from ".git/config"                   
	INFO[0000] origin/origin URL: git@github.com:vbatts/utils.git 
	INFO[0000] appending fetch = +refs/pull/*/head:refs/remotes/pr/* 
	INFO[0000] SUCCESS! `git fetch` and then you can `git checkout pr/$NUM` of your PRs 

