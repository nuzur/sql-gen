-- name: DeleteUser :execresult
DELETE FROM `user`
WHERE
`uuid` = ?;

-- name: DeletePost :execresult
DELETE FROM `post`
WHERE
`uuid` = ?;

