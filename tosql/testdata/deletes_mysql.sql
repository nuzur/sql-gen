-- name: DeleteUser :execresult
DELETE FROM `user`
WHERE
`uuid` = ?;

-- name: DeletePost :execresult
DELETE FROM `post`
WHERE
`uuid` = ?;

-- name: DeleteFolder :execresult
DELETE FROM `folder`
WHERE
`uuid` = ?;

