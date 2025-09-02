FROM grafana/grafana:latest

COPY provisioning /etc/grafana/provisioning
COPY dashboards /var/lib/grafana/dashboards

USER root
RUN chown -R 472:472 /etc/grafana/provisioning
RUN chown -R 472:472 /var/lib/grafana/dashboards
USER 472