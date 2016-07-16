# BoltDB API

## GET requests
`/api/v1/bucket[/:bucket/[:child[/*key]]]`
`/api/v1/containers`
`/api/v1/languages`
`/api/v1/scan[/:bucket/[:child[/*key]]]`
`/api/v1/count/:bucket[/:child]`

Response codes:

- 200 OK
- 400 INVALID REQUEST
- 404 NOT FOUND
- 500 INTERNAL SERVER ERROR

## PUT
Store information into a bucket

`/api/v1/bucket`

Payload:
```
{
    "bucket": "BUCKET NAME",
    "child": "OPTIONAL CHILD BUCKET NAME",
    "key": "NAME OF KEY TO ADD",
    "value": "VALUE OF KEY"
}
```

Response codes:
- 204 NO RESPONSE
- 400 BAD REQUEST

## POST
Create a bucket

`/apì/v1/bucket`

Payload:
```
{
    "bucket": "BUCKET NAME",
    "child": "OPTIONAL CHILD BUCKET NAME"
}
```

Response codes:
- 201 CREATED
- 400 BAD REQUEST

## DELETE
`/api/v1/bucket[/:bucket/[:child[/*key]]]`

Response codes:
- 202 ACCEPTED
- 400 BAD REQUEST
