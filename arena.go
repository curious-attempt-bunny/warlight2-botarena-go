package main

import "fmt"
import "os"
import "os/exec"
import "log"
import "io"
import "bufio"
import "strings"
import "strconv"
import "flag"
import "path/filepath"

type Bot struct {
    name string
    process *exec.Cmd
    stdout *bufio.Reader
    stdin io.WriteCloser
}

type State struct {
    regions map[int64]*Region
    super_regions map[int64]*SuperRegion
    starting_armies int64
}

type Region struct {
    id int64
    super_region *SuperRegion
    neighbours []*Region
    owner string
    armies int64
    visible bool
}

type SuperRegion struct {
    id int64
    regions []*Region
    reward int64
}

type Placement struct {
    region *Region
    armies int64
}

type Movement struct {
    region_from *Region
    region_to *Region
    armies int64
}

func main() {
    flag.Parse()

    launch_command := flag.Arg(0)
    if launch_command == "" {
        log.Fatal("Usage: <bot launcher script>")
    }

    bot := launch(launch_command)
    bot.name = "player1"

    send(bot, "settings timebank 10000")
    send(bot, "settings time_per_move 500")
    send(bot, "settings max_rounds 45")
    send(bot, fmt.Sprintf("settings your_bot %s", bot.name))
    send(bot, "settings opponent_bot player2") // TODO

    // hard-coded map data to get started with
    terrain := []string {
        "setup_map super_regions 1 1 2 0 3 2 4 6 5 1",
        "setup_map regions 1 1 2 1 3 1 4 2 5 2 6 3 7 3 8 3 9 4 10 4 11 4 12 4 13 4 14 4 15 4 16 5 17 5 18 5",
        "setup_map neighbors 1 2,4 2 4,6,3 3 7,6 4 5,6 5 10,9,6 6 7,9,12 7 13,8,12 9 10,12 10 11,14,12,15 11 14 12 15,13 13 15 14 16,15 15 16 16 18,17",
        "setup_map wastelands 1 10" }

    state := &State{}

    for _, line := range terrain {
        send(bot, line)
        state = parse(state, line)
    }

    pick_regions(state, bot, []int64{3, 4, 7, 15, 17})

    for {
        send(bot, "settings starting_armies 5") // TODO

        update_map(state, bot)

        send(bot, "opponent_moves")

        send(bot, "go place_armies 10000")

        placements := placements(state, bot)

        send(bot, "go attack/transfer 10000")

        movements := movements(state, bot)

        _ = placements // TODO
        _ = movements // TODO

        if game_over(state) {
            break
        }
    }
}

func launch(launch_script string) *Bot {
    launch_script, err := filepath.Abs(launch_script)
    if err != nil {
        log.Fatal(err)
    }

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

    cmd.Dir = filepath.Dir(launch_script)

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

func pick_regions(state *State, bot *Bot, regions []int64) {
    // TODO don't hardcode this
    send(bot, "settings starting_regions 3 4 7 15 17")

    remaining_regions := regions

    // simulate that the bot goes first
    remaining_regions = pick_a_region(state, bot, remaining_regions)

    for {
        if len(remaining_regions) == 0 {
            break;
        }

        // simulate the presence of another bot
        remaining_regions = discard_a_region(remaining_regions)
        remaining_regions = discard_a_region(remaining_regions)

        remaining_regions = pick_a_region(state, bot, remaining_regions)
        remaining_regions = pick_a_region(state, bot, remaining_regions)
    }
}

func pick_a_region(state *State, bot *Bot, regions []int64) []int64 {
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

    state.regions[region_id].owner = bot.name

    return new_regions
}

func discard_a_region(regions []int64) []int64 {
    // TODO

    return regions
}

func parse(state *State, line string) *State {
    parts := strings.Split(line, " ")

    if parts[0] == "setup_map" {
        if parts[1] == "super_regions" {
            state.super_regions = make(map[int64]*SuperRegion)
            for i := 2; i < len(parts); i += 2 {
                id, _ := strconv.ParseInt(parts[i], 10, 0)
                reward, _ := strconv.ParseInt(parts[i+1], 10, 0)

                state.super_regions[id] = &SuperRegion{id: id, reward: reward}
            }
        } else if parts[1] == "regions" {
            state.regions = make(map[int64]*Region)
            for i := 2; i < len(parts); i += 2 {
                region_id, _ := strconv.ParseInt(parts[i], 10, 0)
                super_region_id, _ := strconv.ParseInt(parts[i+1], 10, 0)

                super_region := state.super_regions[super_region_id]
                region := &Region{
                    id: region_id,
                    owner: "neutral",
                    super_region: super_region,
                    armies: 2,
                    neighbours: []*Region{} }
                state.regions[region_id] = region
                super_region.regions = append(super_region.regions, region)
            }
        } else if parts[1] == "neighbors" {
            for i := 2; i < len(parts); i += 2 {
                region_id, _ := strconv.ParseInt(parts[i], 10, 0)
                neighbour_ids := strings.Split(parts[i+1], ",")

                region := state.regions[region_id]

                for _, neighbour_id := range neighbour_ids {
                    id, _ := strconv.ParseInt(neighbour_id, 10, 0)
                    neighbour := state.regions[id]
                    region.neighbours = append(region.neighbours, neighbour)
                    neighbour.neighbours = append(neighbour.neighbours, region)
                }
            }
        } else if parts[1] == "wastelands" {
            for i := 2; i < len(parts); i++ {
                region_id, _ := strconv.ParseInt(parts[i], 10, 0)

                region := state.regions[region_id]
                region.armies = 6
            }
        } else {
            log.Fatal(fmt.Sprintf("Don't recognise: %s\n", line))
        }
    } else {
        log.Fatal(fmt.Sprintf("Don't recognise: %s\n", line))
    }

    return state
}

func game_over(state *State) bool {
    return true // TODO
}

func update_map(state *State, bot *Bot) {
    // TODO
}

func placements(state *State, bot *Bot) []*Placement {
    items := []*Placement{}

    return items
}

func movements(state *State, bot *Bot) []*Movement {
    items := []*Movement{}

    return items
}