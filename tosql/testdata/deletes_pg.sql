-- name: DeleteUser :execresult
DELETE FROM "user"
WHERE
"uuid" = ? AND "version" = ?;

-- name: DeleteFolder :execresult
DELETE FROM "folder"
WHERE
"uuid" = ?;

-- name: DeletePost :execresult
DELETE FROM "post"
WHERE
"uuid" = ?;

