# Overview

With botarena you can run your warlight2 bot wherever you want and get fast feedback as to whether or not you've improved it or not.

# Launch convention

botarena does not autodetect the runtime environment for your bot. You must provide a `run.sh` script that knows how to launch your bot.

# Solo play

This will run your bot against a normal map with only neutral enemies. See how many rounds it takes your bot to conquer the entire map. The best we've seen is 7 rounds.

    go run arena.go <path_to_run_script>

# Head to head play

This will run two of your bots against each other.

    go run arena.go <path_to_run_script> <path_to_run_script>

# Game visualisation

This is work in progress. Note that the viewer runs on port 80.

    sudo go run viewer.go
    open http://localhost/competitions/warlight-ai-challenge-2/games/5502423e1c687b0e2fb9df59

# Implementation notes

The fight resolution logic assumes the worst luck for the attacker.