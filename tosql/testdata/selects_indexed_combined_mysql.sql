

-- user selects: 
-- name: FetchUserByUUIDAndVersion :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `uuid` = ? AND `version` = ? ;
        
     
-- name: FetchUserByUUID :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `uuid` = ? 
LIMIT ?, ?;        
     
-- name: FetchUserByEmail :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `email` = ? 
LIMIT ?, ?;        
     
-- name: FetchUserByStatus :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `status` = ? 
LIMIT ?, ?;        
     
-- name: FetchUserByUUIDAndEmail :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `email` = ? AND `uuid` = ? 
LIMIT ?, ?;        
     
-- name: FetchUserByUUIDAndStatus :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `status` = ? AND `uuid` = ? 
LIMIT ?, ?;        
     
-- name: FetchUserByEmailAndStatus :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `email` = ? AND `status` = ? 
LIMIT ?, ?;        
     
-- name: FetchUserByUUIDAndEmailAndStatus :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `email` = ? AND `status` = ? AND `uuid` = ? 
LIMIT ?, ?;        
    
-- name: FetchUserByUUIDAndVersionForUpdate :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `uuid` = ? AND `version` = ? 
FOR UPDATE;
        
-- name: FetchUserByUUIDOrderedByUpdatedAtASC :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `uuid` = ?  
ORDER BY updated_at ASC
LIMIT ?, ?;

-- name: FetchUserByUUIDOrderedByUpdatedAtDESC :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `uuid` = ?  
ORDER BY updated_at DESC
LIMIT ?, ?;

            
-- name: FetchUserByEmailOrderedByUpdatedAtASC :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `email` = ?  
ORDER BY updated_at ASC
LIMIT ?, ?;

-- name: FetchUserByEmailOrderedByUpdatedAtDESC :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `email` = ?  
ORDER BY updated_at DESC
LIMIT ?, ?;

            
-- name: FetchUserByStatusOrderedByUpdatedAtASC :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `status` = ?  
ORDER BY updated_at ASC
LIMIT ?, ?;

-- name: FetchUserByStatusOrderedByUpdatedAtDESC :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `status` = ?  
ORDER BY updated_at DESC
LIMIT ?, ?;

            
-- name: FetchUserByUUIDAndEmailOrderedByUpdatedAtASC :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `email` = ? AND `uuid` = ?  
ORDER BY updated_at ASC
LIMIT ?, ?;

-- name: FetchUserByUUIDAndEmailOrderedByUpdatedAtDESC :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `email` = ? AND `uuid` = ?  
ORDER BY updated_at DESC
LIMIT ?, ?;

            
-- name: FetchUserByUUIDAndStatusOrderedByUpdatedAtASC :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `status` = ? AND `uuid` = ?  
ORDER BY updated_at ASC
LIMIT ?, ?;

-- name: FetchUserByUUIDAndStatusOrderedByUpdatedAtDESC :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `status` = ? AND `uuid` = ?  
ORDER BY updated_at DESC
LIMIT ?, ?;

            
-- name: FetchUserByEmailAndStatusOrderedByUpdatedAtASC :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `email` = ? AND `status` = ?  
ORDER BY updated_at ASC
LIMIT ?, ?;

-- name: FetchUserByEmailAndStatusOrderedByUpdatedAtDESC :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `email` = ? AND `status` = ?  
ORDER BY updated_at DESC
LIMIT ?, ?;

            
-- name: FetchUserByUUIDAndEmailAndStatusOrderedByUpdatedAtASC :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `email` = ? AND `status` = ? AND `uuid` = ?  
ORDER BY updated_at ASC
LIMIT ?, ?;

-- name: FetchUserByUUIDAndEmailAndStatusOrderedByUpdatedAtDESC :many
SELECT `uuid`,`version`,`email`,`password`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `user`
WHERE 
    `email` = ? AND `status` = ? AND `uuid` = ?  
ORDER BY updated_at DESC
LIMIT ?, ?;

            




-- folder selects: 
-- name: FetchFolderByUUID :many
SELECT `uuid`,`version`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `folder`
WHERE 
    `uuid` = ? ;
        
    
-- name: FetchFolderByUUIDForUpdate :many
SELECT `uuid`,`version`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `folder`
WHERE 
    `uuid` = ? 
FOR UPDATE;
        




-- single_key selects: 
-- name: FetchSingleKeyByUUID :many
SELECT `uuid`,`version`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `single_key`
WHERE 
    `uuid` = ? ;
        
     
-- name: FetchSingleKeyByVersion :many
SELECT `uuid`,`version`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `single_key`
WHERE 
    `version` = ? 
LIMIT ?, ?;        
    
-- name: FetchSingleKeyByUUIDForUpdate :many
SELECT `uuid`,`version`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`
FROM `single_key`
WHERE 
    `uuid` = ? 
FOR UPDATE;
        




-- post selects: 
-- name: FetchPostByUUID :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `uuid` = ? ;
        
     
-- name: FetchPostByTitle :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `title` = ? 
LIMIT ?, ?;        
     
-- name: FetchPostBySlug :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `slug` = ? 
LIMIT ?, ?;        
     
-- name: FetchPostByUserUUID :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `user_uuid` = ? 
LIMIT ?, ?;        
     
-- name: FetchPostByStatus :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `status` = ? 
LIMIT ?, ?;        
     
-- name: FetchPostByUserUUIDAndStatus :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `status` = ? AND `user_uuid` = ? 
LIMIT ?, ?;        
     
-- name: FetchPostByTitleAndSlug :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `slug` = ? AND `title` = ? 
LIMIT ?, ?;        
     
-- name: FetchPostByTitleAndUserUUID :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `title` = ? AND `user_uuid` = ? 
LIMIT ?, ?;        
     
-- name: FetchPostBySlugAndUserUUID :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `slug` = ? AND `user_uuid` = ? 
LIMIT ?, ?;        
     
-- name: FetchPostByTitleAndSlugAndUserUUID :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `slug` = ? AND `title` = ? AND `user_uuid` = ? 
LIMIT ?, ?;        
     
-- name: FetchPostByTitleAndStatus :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `status` = ? AND `title` = ? 
LIMIT ?, ?;        
     
-- name: FetchPostBySlugAndStatus :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `slug` = ? AND `status` = ? 
LIMIT ?, ?;        
     
-- name: FetchPostByTitleAndSlugAndStatus :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `slug` = ? AND `status` = ? AND `title` = ? 
LIMIT ?, ?;        
     
-- name: FetchPostByTitleAndUserUUIDAndStatus :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `status` = ? AND `title` = ? AND `user_uuid` = ? 
LIMIT ?, ?;        
     
-- name: FetchPostBySlugAndUserUUIDAndStatus :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `slug` = ? AND `status` = ? AND `user_uuid` = ? 
LIMIT ?, ?;        
     
-- name: FetchPostByTitleAndSlugAndUserUUIDAndStatus :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `slug` = ? AND `status` = ? AND `title` = ? AND `user_uuid` = ? 
LIMIT ?, ?;        
    
-- name: FetchPostByUUIDForUpdate :many
SELECT `uuid`,`version`,`title`,`slug`,`description`,`content`,`status`,`created_at`,`updated_at`,`created_by`,`updated_by`,`media`,`user_uuid`
FROM `post`
WHERE 
    `uuid` = ? 
FOR UPDATE;
        


