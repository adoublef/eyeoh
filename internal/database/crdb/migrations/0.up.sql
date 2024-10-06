create schema fs;

create table fs.dir_entry (
  id uuid
  , root uuid
  , name text not null check (name <> '' and length(name) < 256)
  -- if directory then this value is set as null
  -- ref_id can only be set if sz is also set
  -- , ref uuid
  -- , sz int default 0 check (sz >= 0)
  , mod_at timestamptz default now()
  -- mvcc
  , v int default 0 check (v >= 0)
  , unique (coalesce(root, b'0000000000000000'), name)
  , foreign key (root) references fs.dir_entry (id)
  , primary key (id)
);

create table fs.blob_data (
  -- fka fs.dir_entry (ref)
  id uuid
  , dir_entry uuid
  , sz int default 0 check (sz >= 0)
  -- hash is required
  , sha bytes not null
  , mod_at timestamptz default now()
  -- this will be needed?
  , v int
  , foreign key (dir_entry) references fs.dir_entry (id)
  , primary key (id)
);