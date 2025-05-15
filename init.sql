CREATE TYPE company_type AS ENUM ('Corporations', 'NonProfit', 'Cooperative', 'Sole Proprietorship');

CREATE TABLE companies (
    id UUID DEFAULT gen_random_uuid(),
    name VARCHAR(15) UNIQUE NOT NULL,
    description VARCHAR(3000),
    amount_of_employees INT NOT NULL,
    registered BOOLEAN NOT NULL,
    type company_type NOT NULL,
	PRIMARY KEY(id)
);


CREATE TABLE users (id UUID DEFAULT gen_random_uuid(),
    username VARCHAR(50) UNIQUE NOT NULL,
    password VARCHAR(50),
	PRIMARY KEY (id)
);


insert into users(username,password)values('admin','admin');

insert into users(username,password)values('manager','1234');