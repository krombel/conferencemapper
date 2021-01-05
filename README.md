# Jitsi ConferenceMapper

This may be used as mapping utility between room names and a respecive PIN caller type in when joining via telephone.

You might run this as single binary or as a docker container

    git clone https://github.com/krombel/conferencemapper
    cd conferencemapper
    docker-compose build
    docker-compose up -d

This then listens on Port 8001 so you might reverse proxy it like

    location = /conferenceMapper {
        proxy_pass http://127.0.0.1:8001;
    }

Inspired by https://github.com/luki-ev/conferencemapper. Main differences: sqlite vs redis to store assignments and golang vs python as implementing language.