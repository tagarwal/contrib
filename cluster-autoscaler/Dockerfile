FROM oraclelinux:7

ADD cluster-autoscaler cluster-autoscaler
ADD run.sh run.sh

ADD bmkops /bmkops
WORKDIR /bmkops

COPY epel.repo /etc/yum.repos.d/epel.repo

RUN yum update -y python
RUN yum install -y python-pip python-devel openssl-devel gcc
RUN pip install --upgrade virtualenv pip

RUN virtualenv venv

RUN . venv/bin/activate

RUN pip install -e .

WORKDIR /
CMD ./run.sh