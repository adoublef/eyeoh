create schema up;

create table up.fs (
  id uuid
  , root uuid
  , name text not null check (name <> '' and length(name) < 256)
  -- if directory then this value is set as null
  -- ref_id can only be set if sz is also set
  , ref uuid
  , sz int default 0 check (sz >= 0)
  , mod_at timestamptz default now()
  -- mvcc
  , v int default 0 check (v >= 0)
  , unique (coalesce(root, b'0000000000000000'), name)
  , foreign key (root) references up.fs (id)
  , primary key (id)
);