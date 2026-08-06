

-- user selects: 
-- name: FetchUserByUUIDAndVersion :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "uuid" = ? AND "version" = ? ;
        
     
-- name: FetchUserByUUID :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "uuid" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchUserByEmail :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchUserByStatus :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "status" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchUserByUUIDAndEmail :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ? AND "uuid" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchUserByUUIDAndStatus :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "status" = ? AND "uuid" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchUserByEmailAndStatus :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ? AND "status" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchUserByUUIDAndEmailAndStatus :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ? AND "status" = ? AND "uuid" = ? 
LIMIT ? OFFSET ?;        
    
-- name: FetchUserByUUIDAndVersionForUpdate :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "uuid" = ? AND "version" = ? 
FOR UPDATE;
        
-- name: FetchUserByUUIDOrderedByUpdatedAtASC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "uuid" = ?  
ORDER BY updated_at ASC
LIMIT ? OFFSET ?;

-- name: FetchUserByUUIDOrderedByUpdatedAtDESC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "uuid" = ?  
ORDER BY updated_at DESC
LIMIT ? OFFSET ?;

            
-- name: FetchUserByEmailOrderedByUpdatedAtASC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ?  
ORDER BY updated_at ASC
LIMIT ? OFFSET ?;

-- name: FetchUserByEmailOrderedByUpdatedAtDESC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ?  
ORDER BY updated_at DESC
LIMIT ? OFFSET ?;

            
-- name: FetchUserByStatusOrderedByUpdatedAtASC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "status" = ?  
ORDER BY updated_at ASC
LIMIT ? OFFSET ?;

-- name: FetchUserByStatusOrderedByUpdatedAtDESC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "status" = ?  
ORDER BY updated_at DESC
LIMIT ? OFFSET ?;

            
-- name: FetchUserByUUIDAndEmailOrderedByUpdatedAtASC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ? AND "uuid" = ?  
ORDER BY updated_at ASC
LIMIT ? OFFSET ?;

-- name: FetchUserByUUIDAndEmailOrderedByUpdatedAtDESC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ? AND "uuid" = ?  
ORDER BY updated_at DESC
LIMIT ? OFFSET ?;

            
-- name: FetchUserByUUIDAndStatusOrderedByUpdatedAtASC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "status" = ? AND "uuid" = ?  
ORDER BY updated_at ASC
LIMIT ? OFFSET ?;

-- name: FetchUserByUUIDAndStatusOrderedByUpdatedAtDESC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "status" = ? AND "uuid" = ?  
ORDER BY updated_at DESC
LIMIT ? OFFSET ?;

            
-- name: FetchUserByEmailAndStatusOrderedByUpdatedAtASC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ? AND "status" = ?  
ORDER BY updated_at ASC
LIMIT ? OFFSET ?;

-- name: FetchUserByEmailAndStatusOrderedByUpdatedAtDESC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ? AND "status" = ?  
ORDER BY updated_at DESC
LIMIT ? OFFSET ?;

            
-- name: FetchUserByUUIDAndEmailAndStatusOrderedByUpdatedAtASC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ? AND "status" = ? AND "uuid" = ?  
ORDER BY updated_at ASC
LIMIT ? OFFSET ?;

-- name: FetchUserByUUIDAndEmailAndStatusOrderedByUpdatedAtDESC :many
SELECT "uuid","version","email","password","status","created_at","updated_at","created_by","updated_by"
FROM "user"
WHERE 
    "email" = ? AND "status" = ? AND "uuid" = ?  
ORDER BY updated_at DESC
LIMIT ? OFFSET ?;

            




-- folder selects: 
-- name: FetchFolderByUUID :many
SELECT "uuid","version","status","created_at","updated_at","created_by","updated_by"
FROM "folder"
WHERE 
    "uuid" = ? ;
        
    
-- name: FetchFolderByUUIDForUpdate :many
SELECT "uuid","version","status","created_at","updated_at","created_by","updated_by"
FROM "folder"
WHERE 
    "uuid" = ? 
FOR UPDATE;
        




-- single_key selects: 
-- name: FetchSingleKeyByUUID :many
SELECT "uuid","version","status","created_at","updated_at","created_by","updated_by"
FROM "single_key"
WHERE 
    "uuid" = ? ;
        
     
-- name: FetchSingleKeyByVersion :many
SELECT "uuid","version","status","created_at","updated_at","created_by","updated_by"
FROM "single_key"
WHERE 
    "version" = ? 
LIMIT ? OFFSET ?;        
    
-- name: FetchSingleKeyByUUIDForUpdate :many
SELECT "uuid","version","status","created_at","updated_at","created_by","updated_by"
FROM "single_key"
WHERE 
    "uuid" = ? 
FOR UPDATE;
        




-- post selects: 
-- name: FetchPostByUUID :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "uuid" = ? ;
        
     
-- name: FetchPostByTitle :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "title" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchPostBySlug :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "slug" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchPostByUserUUID :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "user_uuid" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchPostByStatus :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "status" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchPostByUserUUIDAndStatus :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "status" = ? AND "user_uuid" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchPostByTitleAndSlug :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "slug" = ? AND "title" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchPostByTitleAndUserUUID :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "title" = ? AND "user_uuid" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchPostBySlugAndUserUUID :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "slug" = ? AND "user_uuid" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchPostByTitleAndSlugAndUserUUID :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "slug" = ? AND "title" = ? AND "user_uuid" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchPostByTitleAndStatus :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "status" = ? AND "title" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchPostBySlugAndStatus :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "slug" = ? AND "status" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchPostByTitleAndSlugAndStatus :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "slug" = ? AND "status" = ? AND "title" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchPostByTitleAndUserUUIDAndStatus :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "status" = ? AND "title" = ? AND "user_uuid" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchPostBySlugAndUserUUIDAndStatus :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "slug" = ? AND "status" = ? AND "user_uuid" = ? 
LIMIT ? OFFSET ?;        
     
-- name: FetchPostByTitleAndSlugAndUserUUIDAndStatus :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "slug" = ? AND "status" = ? AND "title" = ? AND "user_uuid" = ? 
LIMIT ? OFFSET ?;        
    
-- name: FetchPostByUUIDForUpdate :many
SELECT "uuid","version","title","slug","description","content","status","created_at","updated_at","created_by","updated_by","media","user_uuid"
FROM "post"
WHERE 
    "uuid" = ? 
FOR UPDATE;
        


