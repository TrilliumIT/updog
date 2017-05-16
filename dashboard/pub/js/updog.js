window.setInterval(updateApplications, 5000);

function updateApplications() {
	$.getJSON("/api/applications", function(data) {
		console.log(data);

		$.each(data, function(an, app) {
			if ($('#app_'+an).length == 0) {
				$('#content').append('<div class="application" id="app_'+an+'">'+an+'</div>');
			}
			$.each(app.Services, function(sn, serv) {
				if ($('#serv_'+sn).length == 0) {
					$('#app_'+an).append('<div class="service" id="serv_'+sn+'">'+sn+'</div>');
					$('#serv_'+sn).append('<table><thead><th>Instance Name</th><th>Response Time</th><th>Last Checked</th></thead>');
				}
				$.each(serv.Instances, function(iname, inst) {
					fin = iname.replace(/\/\//g, "__").replace(/:/g, "_").replace(/\./g, "_");
					if ($('#inst_'+fin).length == 0) {
						$('#serv_'+sn+' table').append('<tr class="instance" id="inst_'+fin+'"></tr>');
					}
					if (inst.Up) {
						$('#inst_'+fin).addClass("up");
					}
					$('#inst_'+fin).html('<td>'+iname+'</td><td>'+Math.round(inst.ResponseTime/10000)/100+'</td><td>'+inst.TimeStamp+'</td>');
				});
			});
		});
	});
}

$(function() {
	updateApplications();
});
