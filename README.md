step1:

    install nats-server & natscli
    
    go install github.com/nats-io/nats-server@latest

    go install github.com/nats-io/natscli/nats@latest


step2:

    run nats-server/route1.bat (.\nats-server.exe -c .\\nats-route1.conf)

    run nats-server/route2.bat (.\nats-server.exe -c .\\nats-route2.conf)

    run nats-server/route3.bat (.\nats-server.exe -c .\\nats-route3.conf)

    nats context add local --server "nats://localhost:4222,nats://localhost:4223,nats://localhost:4224" --description "local demo" --user app --password app

    nats context select local

    nats server report jetstream

    http://127.0.0.1:8222/


    cmd1:
        nats reply foo "service instance A Reply# {{Count}}"
    cmd2:
        nats reply foo "service instance B Reply# {{Count}}"
    cmd3:
        nats request foo --count 10 "Request {{Count}}"

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


https://natsbyexample.com/examples/jetstream/partitions/cli