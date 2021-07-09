# -*- bash -*-

sed -i -e 's/10001/10002/' docker-compose.yml
docker-compose up -d
