package dbsync

const ConfigFlagDesc = `configuration file path, also set virtual cwd based on the configuration file directory, if not specify configuration file,
the behaviour is to look up "sync.config.yaml" in parents`

const DryFlagDesc = `run without sync to the database and print the html output (concated with script & stylesheet)`

const ProfileFlagDesc = `specify connection profile to use in configuration file`
