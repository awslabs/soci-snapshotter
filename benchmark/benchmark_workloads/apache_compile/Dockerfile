# READYLINE: /httpd exists

FROM ubuntu:latest

RUN apt-get update
RUN apt-get install -y wget apache2-dev libpcre3 libpcre3-dev
RUN wget http://archive.apache.org/dist/httpd/httpd-2.4.54.tar.gz && tar -xvf httpd-2.4.54.tar.gz
RUN ./httpd-2.4.54/configure -prefix /apacheserver

ADD run.sh /run.sh
CMD ./run.sh
