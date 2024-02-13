run:
	@go run ./webapp

add-user:
	@curl -H "Authorization: my-token" localhost:8080/protected/users --data '{"name":"kkeo", "email":"kkrowa@op.pl"}'

add-token:
	curl -H "Authorization: my-token" localhost:8080/protected/tokens --data '{"name":"main-token", "user_name":"kkeo"}'
