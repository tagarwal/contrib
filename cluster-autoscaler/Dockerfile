FROM oraclelinux:7.3
MAINTAINER Trilok Agarwal "trilok.agarwal@oracle.com"

ADD cluster-autoscaler cluster-autoscaler
ADD run.sh run.sh

COPY bmkops/epel.repo /etc/yum.repos.d/epel.repo

RUN yum update -y python
RUN yum install -y python-pip python-devel openssl-devel gcc
RUN pip install --upgrade virtualenv pip

COPY bmkops /bmkops
WORKDIR /bmkops

RUN virtualenv venv

RUN . venv/bin/activate

RUN pip install -e .

RUN bmkops --help

WORKDIR /

CMD ./run.sh
