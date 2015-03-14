package main

import "bufio"
import "flag"
import "fmt"
import "io"
import "io/ioutil"
import "log"
import "math"
import "os"
import "os/exec"
import "path/filepath"
import "strconv"
import "strings"

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
    starting_regions []int64
    starting_pick_amount int64
    max_rounds int64
    bots []*Bot
    round int64
    data_log *os.File
    player1_log string
    player2_log string
    raw_start []string
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
    var terrain_id *string = flag.String("map", "54f45b994b5ab244fb84c7b1", "Map file to use (default 54f45b994b5ab244fb84c7b1)")
    flag.Parse()

    if len(flag.Args()) != 1 && len(flag.Args()) != 2 {
        log.Fatal("Usage: [-map=<map id>] <bot launcher script> [<bot launcher script>]")
    }

    var filename = "maps/" + *terrain_id + ".txt"

    buffer, err := ioutil.ReadFile(filename)
    terrain := string(buffer)

    if err != nil {
        log.Fatal(err)
    }

    bots := make([]*Bot, len(flag.Args()))

    state := &State{}
    state.bots = bots

    lines := strings.Split(terrain, "\n")

    for _, line := range lines {
        state = parse(state, line)
    }

    for i := 0; i < len(bots); i++ {
        launch_command := flag.Arg(i)
        bot := launch(launch_command)
        bot.id = int64(1+i)
        bot.name = fmt.Sprintf("player%d", bot.id)

        send(bot, "settings timebank 10000") // TODO: remove hardcoded value
        send(bot, "settings time_per_move 500") // TODO: remove hardcoded value
        send(bot, fmt.Sprintf("settings max_rounds %d", state.max_rounds))
        send(bot, fmt.Sprintf("settings your_bot %s", bot.name))
        send(bot, fmt.Sprintf("settings opponent_bot player%d", (3-bot.id)))

        bots[i] = bot
    }

    send_map(state)

    pick_regions(state)

    data_log, err := os.Create("game-data.txt")
    if err != nil {
        log.Fatal(err)
    }
    state.data_log = data_log

    pick_regions(state)

    log_map(state) // NOTE Consistent with theaigames - seems odd, though

    for state.round = 1; state.round <= state.max_rounds+1; state.round++ {
        if game_over(state) {
            break
        } else if state.round == state.max_rounds+1 {
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

func send_map(state *State) {
    for _, bot := range state.bots {
        for i := 0; i < len(state.raw_start); i++ {
            send(bot, state.raw_start[i])
        }
    }
}

func pick_regions(state *State) {
    regions := make([]int64, len(state.starting_regions), len(state.starting_regions))
    copy(regions, state.starting_regions)

    region_strs := make([]string, len(regions))
    for i, id := range regions {
        region_strs[i] = fmt.Sprintf("%d", id)
    }

    for _, bot := range state.bots {
        send(bot, fmt.Sprintf("settings starting_regions %s", strings.Join(region_strs[:], " ")))
        send(bot, fmt.Sprintf("settings starting_pick_amount %d", state.starting_pick_amount))
    }
    log_line(state, strings.Join(region_strs[:], " "))
    log_map(state)
    log_line(state, "round 0")

    remaining_picks := int(state.starting_pick_amount) * len(state.bots)
    remaining_regions := regions

    rotation := []int{0, 1, 1, 0}

    for i := 0; i<remaining_picks; i++ {
        index := rotation[i%4]
        remaining_regions = pick_a_region(state, state.bots[index], remaining_regions)
    }

    for _, bot := range state.bots {
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
    if (strings.TrimSpace(line) == "") {
        return state
    }
    parts := strings.Split(line, " ")

    switch parts[0] {
    case "setup_map":
        state.raw_start = append(state.raw_start, line)
        switch parts[1] {
        case "super_regions":
            state.super_regions = make(map[int64]*SuperRegion)
            for i := 2; i < len(parts); i += 2 {
                id, _ := strconv.ParseInt(parts[i], 10, 0)
                reward, _ := strconv.ParseInt(parts[i+1], 10, 0)

                state.super_regions[id] = &SuperRegion{id: id, reward: reward}
            }
        case "regions":
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
        case "neighbors":
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
        case "wastelands":
            for i := 2; i < len(parts); i++ {
                region_id, _ := strconv.ParseInt(parts[i], 10, 0)

                region := state.regions[region_id]
                region.armies = 6
            }
        default:
            log.Fatal(fmt.Sprintf("Don't recognise: %s\n", line))
        }
    case "settings":
        switch parts[1] {
        case "max_rounds":
            state.max_rounds, _ = strconv.ParseInt(parts[2], 10, 0)
        case "starting_pick_amount":
            state.starting_pick_amount, _ = strconv.ParseInt(parts[2], 10, 0)
        case "starting_regions":
            state.starting_regions = make([]int64, len(parts)-2)
            for i := 2; i < len(parts); i++ {
                state.starting_regions[i-2], _ = strconv.ParseInt(parts[i], 10, 0)
            }
        }
    default:
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
