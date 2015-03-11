package main

import exec "os/exec"

type Bot struct {
    process *exec.Cmd
}

func main() {
    bot := launch("broken_bot.sh")

    send(bot, "settings timebank 10000")
    send(bot, "settings time_per_move 500")
    send(bot, "settings max_rounds 45")
    send(bot, "settings your_bot player1")
    send(bot, "settings opponent_bot player2")

    // hard-coded map data to get started with
    send(bot, "setup_map super_regions 1 1 2 0 3 2 4 6 5 1")
    send(bot, "setup_map regions 1 1 2 1 3 1 4 2 5 2 6 3 7 3 8 3 9 4 10 4 11 4 12 4 13 4 14 4 15 4 16 5 17 5 18 5")
    send(bot, "setup_map neighbors 1 2,4 2 4,6,3 3 7,6 4 5,6 5 10,9,6 6 7,9,12 7 13,8,12 9 10,12 10 11,14,12,15 11 14 12 15,13 13 15 14 16,15 15 16 16 18,17")
    send(bot, "setup_map wastelands 1 10")

    pick_regions(bot, []int64{3, 4, 7, 15, 17})
}

func launch(launch_script string) *Bot {
    cmd := exec.Command(launch_script)

    bot := &Bot{process: cmd}

    return bot
}

func send(bot *Bot, command string) {
    // TODO
}

func pick_regions(bot *Bot, regions []int64) {
    // TODO don't hardcode this
    send(bot, "settings starting_regions 3 4 7 15 17")

    remaining_regions := regions

    // simulate that the bot goes first
    remaining_regions = pick_a_region(bot, remaining_regions)

    for {
        if len(remaining_regions) == 0 {
            break;
        }

        // simulate the presence of another bot
        remaining_regions = discard_a_region(remaining_regions)
        remaining_regions = discard_a_region(remaining_regions)

        remaining_regions = pick_a_region(bot, remaining_regions)
        remaining_regions = pick_a_region(bot, remaining_regions)
    }
}

func pick_a_region(bot *Bot, regions []int64) []int64 {
    // TODO

    return regions
}

func discard_a_region(regions []int64) []int64 {
    // TODO

    return regions
}
