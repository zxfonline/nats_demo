step1:

    install nats-server & tool
    

    export GO111MODULE=on
    go install github.com/nats-io/nats-server/v2@latest
    go install github.com/nats-io/natscli/nats@latest
    go install github.com/nats-io/nkeys/nk@latest
    go install github.com/nats-io/nsc/v2@latest


step2:

    start cluster1.bat
    start cluster2.bat
   
    nats context add cluster1 --server nats://localhost:4222,nats://localhost:4223,nats://localhost:4224 --user app --password app  --description "cluster1"

    nats context add cluster2 --server nats://localhost:4322,nats://localhost:4323,nats://localhost:4324 --user app --password app  --description "cluster2"

    nats context add super_cluster --server nats://localhost:4222,nats://localhost:4322 --user admin --password admin  --description "super cluster" --select

    nats context select cluster1
    nats context select cluster2
    nats context select super_cluster

    nats server report jetstream

    http://127.0.0.1:8222/

    http://127.0.0.1:8322/


    check diff:

        nats reply foo "service instance A Reply# {{Count}}"
        nats reply foo "service instance B Reply# {{Count}}"
        nats request foo --count 10 "Request {{Count}}"
        nats request foo --count 10 "Request {{Count}}"

    check diff:

        nats -s "nats://app:app@localhost:4222" sub "foo"
        nats -s "nats://app:app@localhost:4223" sub "foo"
        nats -s "nats://app:app@localhost:4322" sub "foo"
        nats -s "nats://app:app@localhost:4323" sub "foo"

        nats -s "nats://app:app@localhost:4222" pub foo bar --count 10
        nats -s "nats://app:app@localhost:4223" pub foo bar --count 10

        nats -s "nats://app:app@localhost:4322" pub foo bar --count 10
        nats -s "nats://app:app@localhost:4323" pub foo bar --count 10

    left local to remote:

        nats-server.exe -c .\leaf_remote.conf

        nats reply -s nats://localhost:4411 q 42
        nats reply -s nats://app:app@localhost:4411 q 42


        nats-server.exe -c .\leaf_local.conf

        nats req -s nats://127.0.0.1:4422 q "req remote leftnode"
        nats req -s nats://app:app@127.0.0.1:4422 q "req remote leftnode"


        https://docs.nats.io/running-a-nats-service/configuration/leafnodes/jetstream_leafnodes
    
    nats -s "nats://app:app@localhost:4222" sub "pevents.0.*"
    nats -s "nats://app:app@localhost:4222" sub "pevents.1.*"
    nats -s "nats://app:app@localhost:4222" sub "pevents.2.*"

    nats -s "nats://app:app@localhost:4322" pub pevents.a "Request {{Count}}" --count 10
    nats -s "nats://app:app@localhost:4322" pub pevents.e "Request {{Count}}" --count 10
    nats -s "nats://app:app@localhost:4322" pub pevents.g "Request {{Count}}" --count 10

natsboard:

    https://github.com/devfacet/natsboard


nats-tools:

    https://docs.nats.io/using-nats/nats-tools

    go install github.com/nats-io/nats-top


example:

    https://docs.nats.io/ ***

    https://natsbyexample.com/

    https://natsbyexample.com/examples/jetstream/

    https://www.bookstack.cn/read/NATS-2.8-en/welcome.md

    https://github.com/nats-io/natscli


    https://natsbyexample.com/examples/jetstream/partitions/cli