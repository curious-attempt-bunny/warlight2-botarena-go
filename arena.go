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
import "math"

type Bot struct {
    id int64
    name string
    process *exec.Cmd
    stdout *bufio.Reader
    stdin io.WriteCloser
}

type State struct {
    regions map[int64]*Region
    super_regions map[int64]*SuperRegion
    starting_armies int64
    bots []*Bot
    round int
    data_log *os.File
    player1_log string
    player2_log string
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

    if len(flag.Args()) != 1 && len(flag.Args()) != 2 {
        log.Fatal("Usage: <bot launcher script> (<bot launcher script>)")
    }

    bots := make([]*Bot, len(flag.Args()))

    // hard-coded map data to get started with (from map 54f45b994b5ab244fb84c7b1)
    terrain := []string {
        "setup_map super_regions 1 3 2 2 3 2 4 2 5 5 6 4 7 3 8 5 9 5 10 4 11 5 12 5 13 2",
        "setup_map regions 1 1 2 1 3 1 4 1 5 1 6 2 7 2 8 2 9 3 10 3 11 3 12 4 13 4 14 4 15 5 16 5 17 5 18 5 19 5 20 5 21 6 22 6 23 6 24 6 25 6 26 7 27 7 28 7 29 7 30 8 31 8 32 8 33 8 34 8 35 9 36 9 37 9 38 9 39 9 40 9 41 10 42 10 43 10 44 10 45 11 46 11 47 11 48 11 49 11 50 12 51 12 52 12 53 12 54 12 55 12 56 12 57 13 58 13 59 13",
        "setup_map neighbors 1 2,4,3 2 7,4 3 4,5,32 4 7,33,5 5 32,33 6 7,8 7 33,22,21 8 21 9 10,11 10 11,14,12,13 11 35,14,36,42 12 13 13 27,14,26 14 43,28,42,27 15 16 16 18,36,17,19,30,31 17 31,19 18 36,19 19 38,40,37,34,36,31,57,20 20 40,57,58 21 22 22 23,24,33 23 46,48,33,24,25,45,52,53 24 51,52 25 53,48 26 41,27 27 41,28,29,44 28 43,29 29 43,44 30 31 31 32,34 32 34,33,45 33 45 34 47,57,45 35 36 36 42,38,37 37 40,39,38,42,43 39 42,43 41 44 42 43 43 44 45 47,46 46 48 47 49,59,57,58 48 49,53 49 53,59 50 51,54 51 52,54 52 53,56,54 53 56 54 56,55 57 58 58 59",
        "setup_map wastelands 12 20 24 25 31 58" }
    for i := 0; i < len(bots); i++ {
        launch_command := flag.Arg(i)
        bot := launch(launch_command)
        bot.id = int64(1+i)
        bot.name = fmt.Sprintf("player%d", bot.id)

        send(bot, "settings timebank 10000")
        send(bot, "settings time_per_move 500")
        send(bot, "settings max_rounds 147")
        send(bot, fmt.Sprintf("settings your_bot %s", bot.name))
        send(bot, fmt.Sprintf("settings opponent_bot player%d", (3-bot.id)))

        for _, line := range terrain {
            send(bot, line)
        }

        bots[i] = bot
    }

    state := &State{}
    state.bots = bots

    for _, line := range terrain {
        state = parse(state, line)
    }

    data_log, err := os.Create("game-data.txt")
    if err != nil {
        log.Fatal(err)
    }
    state.data_log = data_log

    pick_regions(state, bots, []int64{5, 8, 10, 13, 17, 23, 27, 30, 38, 41, 48, 54, 57})

    log_map(state) // NOTE Consistent with theaigames - seems odd, though

    for state.round = 1; state.round <= 147+1; state.round++ { // TODO
        if game_over(state) {
            break
        } else if state.round == 147+1 {
            fmt.Println("DRAW GAME")
            break
        }

        log_line(state, fmt.Sprintf("round %d", state.round))

        fmt.Println()
        fmt.Printf("-- Round %d\n", state.round)

        placements := make([][]*Placement, len(bots))
        movements := make([][]*Movement, len(bots))

        for i, bot := range bots {
            send(bot, fmt.Sprintf("settings starting_armies %d", starting_armies(state, bot)))

            update_map(state, bot)

            send(bot, "opponent_moves")

            send(bot, "go place_armies 10000")

            placements[i] = recieve_placements(state, bot)

            send(bot, "go attack/transfer 10000")

            movements[i] = recieve_movements(state, bot)
        }

        state = apply(state, placements, movements)
    }

    io.WriteString(state.data_log, state.player1_log)
    io.WriteString(state.data_log, state.player2_log)
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
    fmt.Fprintf(os.Stderr, ">%d> %s\n", bot.id, command)
    io.WriteString(bot.stdin, fmt.Sprintf("%s\n", command))
}

func receive(bot *Bot) string {
    line, _ := bot.stdout.ReadString('\n')
    line = strings.TrimSpace(line)

    fmt.Fprintf(os.Stderr, "<%d< %s\n", bot.id, line)

    return line
}

func log_line(state *State, line string) {
    io.WriteString(state.data_log, line+"\n")
    state.player1_log += line+"\n"
    state.player2_log += line+"\n"
}

func log_map(state *State) {
    io.WriteString(state.data_log, render_map(state, nil))

    state.player1_log += render_map(state, state.bots[0])
    state.player2_log += render_map(state, state.bots[1])
}

func render_map(state *State, bot *Bot) string {
    rendered_map := "map"

    for i := 1; i <= len(state.regions); i++ {
        region := state.regions[int64(i)]
        visible := false
        if bot == nil {
            visible = true
        } else {
            for _, neighbour := range region.neighbours {
                if neighbour.owner == bot.name {
                    visible = true
                    break
                }
            }
        }

        if visible {
            rendered_map += fmt.Sprintf(" %d;%s;%d", region.id, region.owner, region.armies)
        } else {
            rendered_map += fmt.Sprintf(" %d;owner;0", region.id)
        }
    }
    rendered_map += "\n"

    return rendered_map
}

func pick_regions(state *State, bots []*Bot, regions []int64) {
    for _, bot := range bots {
        send(bot, "settings starting_regions 5 8 10 13 17 23 27 30 38 41 48 54 57") // TODO
        send(bot, "settings starting_pick_amount 4") // TODO
    }
    log_line(state, "5 8 10 13 17 23 27 30 38 41 48 54 57") // TODO
    log_map(state)
    log_line(state, "round 0")

    remaining_picks := 4*len(bots) // TODO
    remaining_regions := regions

    rotation := []int{0, 1, 1, 0}

    for i := 0; i<remaining_picks; i++ {
        index := rotation[i%4]
        remaining_regions = pick_a_region(state, bots[index], remaining_regions)
    }

    for _, bot := range bots {
        send(bot, "setup_map opponent_starting_regions") // TODO
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

    log_line(state, fmt.Sprintf("%s place_armies %d %d", bot.name, region_id, state.regions[region_id].armies))
    log_map(state)

    return new_regions
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
    countByOwner := make(map[string]int64)

    for _, region := range state.regions {
        countByOwner[region.owner] += 1
    }

    if len(state.bots) == 1 {
        if countByOwner["neutral"] == 0 {
            fmt.Printf("CONQUEST IN %d ROUNDS", state.round)
            log_line(state, "Nobody won")
            return true
        }
    } else {
        if countByOwner["player1"] == 0 || countByOwner["player2"] == 0 {
            winner := "player1"
            if countByOwner["player1"] == 0 {
                winner = "player2"
            }

            fmt.Printf("WIN BY %s IN %d ROUNDS\n", winner, state.round)
            log_line(state, fmt.Sprintf("%s won", winner))
            return true
        }
    }

    return false
}

func update_map(state *State, bot *Bot) {
    output := "update_map"
    for _, region := range state.regions {
        visible := region.owner == bot.name
        if !visible {
            for _, neighbour := range region.neighbours {
                if neighbour.owner == bot.name {
                    visible = true
                    break
                }
            }
        }

        if visible {
            output = fmt.Sprintf("%s %d %s %d", output, region.id, region.owner, region.armies)
        }
    }
    send(bot, output)
}

func recieve_placements(state *State, bot *Bot) []*Placement {
    items := []*Placement{}

    line := receive(bot)

    if line == "No moves" {
        return items
    }

    commands := strings.Split(line, ",")

    remaining_armies := starting_armies(state, bot)

    for _, command := range commands {
        command := strings.TrimSpace(command)
        if command == "" {
            continue
        }
        parts := strings.Split(command, " ")
        if !(len(parts) == 4 && parts[1] == "place_armies" && parts[0] == bot.name) {
            log.Fatal(fmt.Sprintf("Wrong placement format: %s", command))
        }

        region_id, _ := strconv.ParseInt(parts[2], 10, 0)
        armies, _ := strconv.ParseInt(parts[3], 10, 0)

        region := state.regions[region_id]

        if armies <= 0 {
            log.Fatal(fmt.Sprintf("Must place a positive number of armies: %s", command))
        }

        if armies > remaining_armies {
            log.Fatal(fmt.Sprintf("Trying to place more armies than are available: %s", command))
        }

        if region.owner != bot.name {
            log.Fatal(fmt.Sprintf("Must place armies on an owned region: %s", command))
        }

        placement := &Placement{
            region: region,
            armies: armies}

        items = append(items, placement)

        remaining_armies -= armies
    }

    if remaining_armies > 0 {
        log.Fatal("Did not place all armies available")
    }

    return items
}

func recieve_movements(state *State, bot *Bot) []*Movement {
    items := []*Movement{}

    line := receive(bot)

    if line == "No moves" {
        return items
    }

    commands := strings.Split(line, ",")

    for _, command := range commands {
        command := strings.TrimSpace(command)
        if command == "" {
            continue
        }
        parts := strings.Split(command, " ")
        if !(len(parts) == 5 && parts[1] == "attack/transfer" && parts[0] == bot.name) {
            log.Fatal(fmt.Sprintf("Wrong placement format: %s", command))
        }

        from_id, _ := strconv.ParseInt(parts[2], 10, 0)
        to_id, _ := strconv.ParseInt(parts[3], 10, 0)
        armies, _ := strconv.ParseInt(parts[4], 10, 0)

        region_from := state.regions[from_id]
        region_to := state.regions[to_id]

        if armies <= 0 {
            log.Fatal(fmt.Sprintf("Must move a positive number of armies: %s", command))
        }

        // Yes.  It's sensible to "attack" with one army if your bot considers it to have been captured with a previous attack.
        // if region_to.owner != bot.name && armies == 1 {
        //     log.Fatal(fmt.Sprintf("Trying to attack with just one army: %s", command))
        // }

        if region_from.owner != bot.name {
            log.Println(fmt.Sprintf("Must own the source region at the start of the turn: %s", command))
            continue
        }

        movement := &Movement{
            region_from: region_from,
            region_to: region_to,
            armies: armies}

        items = append(items, movement)
    }

    return items
}

func apply(state *State, placements [][]*Placement, movements [][]*Movement) *State {
    for i, _ := range state.bots {
        for _, placement := range placements[i] {
            placement.region.armies += placement.armies
            log_line(state, fmt.Sprintf("%s place_armies %d %d",
                state.bots[i].name, placement.region.id, placement.armies))
            log_map(state)
        }
    }

    movement_count := 0
    for i, _ := range state.bots {
        movement_count += len(movements[i])
    }

    rotation := []int{0, 1, 1, 0}
    indexes := []int{0, 0}
    all_movements := make([]*Movement, movement_count)
    for j, _ := range all_movements {
        bot_index := rotation[j%4]
        if indexes[bot_index] == len(movements[bot_index]) {
            bot_index = (bot_index + 1) % len(state.bots)
        }
        all_movements[j] = movements[bot_index][indexes[bot_index]]
        indexes[bot_index]++
    }

    for _, movement := range all_movements {
        if movement.region_from.armies <= movement.armies {
            // log.Fatal("Trying to move more armies than remain")
            movement.armies = movement.region_from.armies-1
        }

        if movement.region_to.owner == movement.region_from.owner {
            movement.region_to.armies += movement.armies
            movement.region_from.armies -= movement.armies
        } else {
            // Attack minimums to win (worst luck):
            // 2x1 -- (-1) --> 1
            // 3x2 -- (-1) --> 2
            // 5x3 -- (-2) --> 3
            // 7x4 -- (-3) --> 4
            // 9x5 -- (-4) --> 5
            // 11x6 -- (-4) --> 7
            // 13x7 -- (-5) --> 8
            // 15x8 -- (-6) --> 9
            // 17x9 -- (-7) --> 10
            defending_armies_killed := (movement.armies+1)/2
            if movement.armies < 2 {
                defending_armies_killed = 0
            } else if movement.armies == 2 {
                defending_armies_killed = 1
            }

            defending_armies := float64(movement.region_to.armies)
            luck := float64(0.16)
            killed := ((0.7 * defending_armies) * (1 - luck)) + (defending_armies * luck)
            attacking_armies_killed := int64(math.Ceil(killed-0.5))
            if attacking_armies_killed > movement.armies {
                attacking_armies_killed = movement.armies
            }

            // fmt.Printf("%dx%d -- (-%d vs %g) --> %d\n", movement.armies, movement.region_to.armies, attacking_armies_killed, killed, movement.armies - attacking_armies_killed)

            if defending_armies_killed < movement.region_to.armies {
                // lost the attack

                movement.region_to.armies -= defending_armies_killed
                if movement.region_to.armies < 1 {
                    movement.region_to.armies = 1
                }
                movement.region_from.armies -= attacking_armies_killed
            } else if attacking_armies_killed == movement.armies {
                // won the attack but no armies left to capture with

                movement.region_to.armies = 1
                movement.region_from.armies -= attacking_armies_killed
            } else {
                // won the attack

                movement.region_to.owner = movement.region_from.owner
                movement.region_to.armies = movement.armies - attacking_armies_killed
                movement.region_from.armies -= movement.armies
            }
        }

        log_line(state, fmt.Sprintf("%s attack/transfer %d %d %d",
            movement.region_from.owner, movement.region_from.id, movement.region_to.id, movement.armies))
        log_map(state)
    }

    return state
}

func starting_armies(state *State, bot *Bot) int64 {
    armies := int64(5)

    for _, super_region := range state.super_regions {
        complete := true
        for _, subregion := range super_region.regions {
            if subregion.owner != bot.name {
                complete = false
                break
            }
        }

        if complete {
            armies += super_region.reward
        }
    }

    return armies
}
