package main

import "fmt"
import "os"
import "os/exec"
import "log"
import "io"
import "bufio"
import "strings"
import "strconv"

type Bot struct {
    process *exec.Cmd
    stdout *bufio.Reader
    stdin io.WriteCloser
}

func main() {
    bot := launch("./fake_bot.sh")

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
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        log.Fatal(err)
    }
    stdin, err := cmd.StdinPipe()
    if err != nil {
        log.Fatal(err)
    }

    output := bufio.NewReader(stdout)

    err = cmd.Start()
    if err != nil {
        log.Fatal(err)
    }

    bot := &Bot{process: cmd, stdin: stdin, stdout: output}

    return bot
}

func send(bot *Bot, command string) {
    fmt.Fprintf(os.Stderr, ">> %s\n", command)
    io.WriteString(bot.stdin, fmt.Sprintf("%s\n", command))
}

func receive(bot *Bot) string {
    line, _ := bot.stdout.ReadString('\n')
    line = strings.TrimSpace(line)

    fmt.Fprintf(os.Stderr, "<< %s\n", line)

    return line
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
    region_strs := make([]string, len(regions))
    for i, id := range regions {
        region_strs[i] = fmt.Sprintf("%d", id)
    }
    send(bot, fmt.Sprintf("pick_starting_region 10000 %s", strings.Join(region_strs[:], " ")))

    line := receive(bot)

    region_id, err := strconv.ParseInt(line, 10, 0)
    if err != nil {
        log.Fatal(err)
    }
    index := -1
    for i, id := range regions {
        if id == region_id {
            index = i
            break
        }
    }
    if index == -1 {
        log.Fatal(fmt.Sprintf("Not a valid choice of starting region: %s\n", line))
    }

    new_regions := make([]int64, len(regions)-1)
    copy(new_regions[:], regions[0:index])
    copy(new_regions[index:], regions[index+1:])

    return new_regions
}

func discard_a_region(regions []int64) []int64 {
    // TODO

    return regions
}
