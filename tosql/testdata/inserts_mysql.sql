-- name: InsertUser :execresult
INSERT INTO `user`
(`uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`)
VALUES
(?,?,?,?,?,?,?,?,?);

-- name: InsertPost :execresult
INSERT INTO `post`
(`uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`)
VALUES
(?,?,?,?,?,?,?,?,?,?,?,?,?);

