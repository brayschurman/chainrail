package cmd

func commandRegistered(name string) bool {
	for _, c := range rootCmd.Commands() {
		if c.Name() == name {
			return true
		}
	}
	return false
}
