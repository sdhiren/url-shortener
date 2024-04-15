create  table  URLS  (  
    ID  SERIAL PRIMARY KEY,  
    LONG_URL  varchar(1000)  not  null,
    SHORT_URL  varchar(20)  not  null,
    COUNTER_VALUE  int not  null
);