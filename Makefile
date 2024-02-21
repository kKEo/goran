TOKEN?=letmein_IknowTheSecret

run:
	@go run ./webapp

add-user:
	@curl -H "Authorization: $(TOKEN)" localhost:8080/protected/users --data '{"name":"kkeo", "email":"kkrowa@op.pl"}'

show-users:
	@curl -H "Authorization: $(TOKEN)"  localhost:8080/protected/users

add-token:
	@curl -H "Authorization: $(TOKEN)" localhost:8080/protected/tokens --data '{"name":"main-token", "user_name":"kkeo"}'

ME?=kkeo
show-my-tokens:
	@curl -H "Authorization: $(TOKEN)"  localhost:8080/protected/users/$(ME)/tokens

add-blueprint:
	@curl -H "Authorization: $(TOKEN)" localhost:8080/protected/blueprints --data '{"name": "my-cluster-22", "machine": "g6-standard-2", "api_token": "token"}'

get-blueprint:
	@curl -H "Authorization: $(TOKEN)" localhost:8080/protected/blueprints/my-cluster
