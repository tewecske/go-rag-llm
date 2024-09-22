# Install redis stack
curl -fsSL https://packages.redis.io/gpg | sudo gpg --dearmor -o /usr/share/keyrings/redis-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/redis-archive-keyring.gpg] https://packages.redis.io/deb $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/redis.list

 2589  sudo apt updatesudo apt install redis-stack-server
sudo systemctl enable redis-stack-server
sudo systemctl start redis-stack-server
# test
redis-cli -h 127.0.0.1 -p 6379 PING


# Install ollama
curl http://localhost:11434/api/pull -d '{ "name": "gemma:2b" }'

go run . --load
go run . --query "Question about data"

# Prepare data from online school books
curl https://www.nkp.hu/api/get_book_structure?book_uri_segment=tortenelem_08_nat2020 > tori8_structure.json
cat tori8_structure.json | jq -r '.lessons[] | .lesson_id' > tori8_lessons.txt
cat tori8_lessons.txt | while read -a lid; do echo $lid; curl "https://www.nkp.hu/api/get_book_lesson_content?id=$lid&published=true" > tori8_$lid.json; done
cat tori8_lessons.txt | while read -a lid; do echo $lid; jq -r '.renderedSections[] | .name,(.values.content | gsub("<[^>]+>"; ""))' tori8_$lid.json > tori8_$lid.txt; done
find ./data/tori/ -type f | while read -a tori; do echo $tori; go run . --load "$tori"; done
# Test
go run . --query "Az ókori görögök mit akartak Athénban a cserépszavazással elkerülni?"
go run . --query "Miről vált híressé Kőrösi Csoma Sándor?"


