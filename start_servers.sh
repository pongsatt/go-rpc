# run kafka compatible server
docker run --name redpanda -p 9092:9092 vectorized/redpanda:v21.4.1

# create topics for example
docker exec -it redpanda rpk topic consume RealServer-request
docker exec -it redpanda rpk topic create RealServer-reply
