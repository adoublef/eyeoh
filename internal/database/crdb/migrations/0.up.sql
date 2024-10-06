create schema fs;

create table fs.dir_entry (
  id uuid
  , root uuid
  , name text not null check (name <> '' and length(name) < 256)
  , mod_at timestamptz default now()
  -- mvcc (i.e. renaming, moving or updating blob data)
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
  -- (dir_entry can refernce this for history)
  , v int
  , foreign key (dir_entry) references fs.dir_entry (id)
  , primary key (id)
);