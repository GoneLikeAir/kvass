FROM uat.sf.dockerhub.stgwebank/common/ubuntu:2024061419
COPY kvass /kvass

ENTRYPOINT ["/kvass"]
