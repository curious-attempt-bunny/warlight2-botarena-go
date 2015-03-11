package main

import exec "os/exec"

type Bot struct {
    process *exec.Cmd
}

func main() {
    launch("broken_bot.sh")
}

func launch(launch_script string) *Bot {
    cmd := exec.Command(launch_script)

    bot := &Bot{process: cmd}

    return bot
}
