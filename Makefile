run:
	@go run ./webapp

add-user:
	@curl -H "Authorization: my-token" localhost:8080/protected/users --data '{"name":"kkeo", "email":"kkrowa@op.pl"}'
