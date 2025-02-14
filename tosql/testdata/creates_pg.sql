CREATE TABLE IF NOT EXISTS "user" (
    "uuid" UUID NOT NULL,
    "version" INTEGER NOT NULL,
    "email" VARCHAR(512),
    "password" VARCHAR(255),
    "status" INTEGER NOT NULL,
    "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_by" UUID NOT NULL,
    "updated_by" UUID NOT NULL,
    PRIMARY KEY ("uuid"),
    UNIQUE ("uuid"),
    UNIQUE ("email"))

CREATE TABLE IF NOT EXISTS "post" (
    "uuid" UUID NOT NULL,
    "version" INTEGER NOT NULL,
    "title" VARCHAR(255) NOT NULL,
    "slug" VARCHAR(512) NOT NULL,
    "description" VARCHAR(255),
    "content" TEXT,
    "status" INTEGER NOT NULL,
    "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_by" UUID NOT NULL,
    "updated_by" UUID NOT NULL,
    "media" JSON NOT NULL,
    "user_uuid" UUID NOT NULL,
    PRIMARY KEY ("uuid"),
    UNIQUE ("slug"),
    CONSTRAINT "post_user"
        FOREIGN KEY ("user_uuid")
        REFERENCES "user" ("uuid"))

