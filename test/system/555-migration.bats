#!/usr/bin/env bats

load helpers

@test "Podman - BoltDB to SQLite migration" {
    skip_if_remote "migration is only possible with local Podman"

    # Force BoltDB
    safe_opts=$(podman_isolation_opts ${PODMAN_TMPDIR})
    CI_DESIRED_DATABASE=boltdb run_podman $safe_opts --db-backend=boltdb info
    export SUPPRESS_BOLTDB_WARNING=true

    run_podman info $safe_opts --format '{{.Host.DatabaseBackend}}'
    is "$output" "boltdb"

    # Create objects to migrate
    volume_1_name="myvol"
    volume_2_name="myvol2"
    run_podman $safe_opts volume create $volume_1_name
    run_podman $safe_opts volume create $volume_2_name

    pod_name="mypod"
    run_podman $safe_opts pod create $pod_name

    ctr_1_name="myctr"
    ctr_2_name="myctr2"
    run_podman $safe_opts create --name $ctr_1_name --pod $pod_name --volume $volume_1_name:/test $IMAGE ls /
    run_podman $safe_opts create --name $ctr_2_name --volume $volume_2_name:/test $IMAGE ls /

    # Gather data to compare after migration
    run_podman $safe_opts volume ls -q
    volumes="$output"
    run_podman $safe_opts pod ps -q
    pods="$output"
    run_podman $safe_opts ps -aq
    ctrs="$output"

    run_podman $safe_opts volume inspect $volume_1_name
    v1_inspect="$output"
    run_podman $safe_opts volume inspect $volume_2_name
    v2_inspect="$output"
    run_podman $safe_opts pod inspect $pod_name
    pod_inspect="$output"
    run_podman $safe_opts container inspect $ctr_1_name
    ctr1_inspect="$output"
    run_podman $safe_opts container inspect $ctr_2_name
    ctr2_inspect="$output"

    # Perform migration
    if [[ "$CI_DESIRED_DATABASE" == "boltdb" ]]; then
        # Podman will print an extra warning here because containers.conf is set to boltdb
        run_podman 125 $safe_opts system migrate --migrate-db
        assert "$output" =~ "Error: unable to migrate to SQLite database as database backend manually set" "migration error due to manually set database backend"

        run_podman $safe_opts rm -af
        run_podman $safe_opts pod rm -af
        run_podman $safe_opts volume rm -af
        run_podman $safe_opts rmi -af

        skip "Remainder of test requires successful migration"
    else
        run_podman $safe_opts system migrate --migrate-db
    fi

    # Ensure it took affect
    run_podman info $safe_opts --format '{{.Host.DatabaseBackend}}'
    assert "$output" == "sqlite"

    # Ensure containers, pods, volumes migrated correctly
    run_podman $safe_opts volume ls -q
    assert "$output" == "$volumes"
    run_podman $safe_opts pod ps -q
    assert "$output" == "$pods"
    run_podman $safe_opts ps -aq
    is "$output" "$ctrs"

    run_podman $safe_opts volume inspect $volume_1_name
    assert "$output" == "$v1_inspect"
    run_podman $safe_opts volume inspect $volume_2_name
    assert "$output" == "$v2_inspect"
    run_podman $safe_opts pod inspect $pod_name
    assert "$output" == "$pod_inspect"
    run_podman $safe_opts container inspect $ctr_1_name
    assert "$output" == "$ctr1_inspect"
    run_podman $safe_opts container inspect $ctr_2_name
    assert "$output" == "$ctr2_inspect"

    unset SUPPRESS_BOLTDB_WARNING

    run_podman $safe_opts rm -af
    run_podman $safe_opts pod rm -af
    run_podman $safe_opts volume rm -af
    run_podman $safe_opts rmi -af
}
