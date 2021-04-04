# run kafka compatible server
docker run --rm \
--name=redpanda \
-p 9092:9092 \
"vectorized/redpanda:v21.4.1" redpanda start \
--smp 1  \
--memory 1G  \
--reserve-memory 0M \
--overprovisioned \
--node-id 0 \
--check=false \
--kafka-addr 0.0.0.0:9092 \
--advertise-kafka-addr 127.0.0.1:9092 \
--rpc-addr 0.0.0.0:33145 \
--advertise-rpc-addr redpanda:33145 \
-- \
--kernel-page-cache 1

# create topics for example
docker exec -it redpanda rpk topic consume RealServer-request
docker exec -it redpanda rpk topic create RealServer-reply
