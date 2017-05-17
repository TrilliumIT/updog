window.setInterval(updateApplications, 5000);
window.setInterval(updateTimestamps, 1000);

function updateApplications() {
	$.getJSON("/api/applications", function(data) {
		console.log(data);
		$.each(data, function(an, app) {
			if ($('#app_'+an).length == 0) {
				$('#content').append('<div class="application" id="app_'+an+'"><span class="title">'+an+'</span></div>');
			}
			$.each(app.Services, function(sn, serv) {
				if ($('#serv_'+an+'_'+sn).length == 0) {
					$('#app_'+an).append('<div class="service" id="serv_'+an+'_'+sn+'"><div class="serv_sum"><span class="title">'+sn+'</span></div></div>');
					$('#serv_'+an+'_'+sn).append('<div class="inst_table"><table><thead><th class="inh">Instance Name</th><th class="rth">Response Time</th><th class="lch">Last Checked</th></thead><tbody></tbody></div>');
				}

				var tot_rt = 0

				$.each(serv.Instances, function(iname, inst) {

					var fin = iname.replace(/\/\//g, "__").replace(/:/g, "_").replace(/\./g, "_");
					if ($('#inst_'+fin).length == 0) {
						$('#serv_'+an+'_'+sn+' table tbody').append('<tr class="instance" id="inst_'+fin+'"></tr>');
					}
					if (inst.Up) {
						$('#inst_'+fin).addClass("up");
					}
					$('#inst_'+fin).html('<td class="ind">'+iname+'</td><td class="rtd">'+toMsFormatted(inst.ResponseTime)+'</td><td class="lcd"><time title="'+inst.TimeStamp+'" ></time></td>');
					tot_rt += inst.ResponseTime
				});

					var avg_rt = tot_rt / (serv.Down + serv.Up)
			});
		});
		updateTimestamps();
	});
}

$(function() {
	updateApplications();

	$('.content').on("click", '.service', function(event) {
		$(event.target).children('div.inst_table').slideToggle();
	});

	$('.content').on("click", '.serv_sum', function(event) {
		$(event.target).parent().trigger("click");
	});

});

function toMsFormatted(number) {
	return (Math.round(number/10000)/100).toFixed(2);
}

function updateTimestamps() {
	$('time').each(function() {
		$(this).text((moment().unix() - moment($(this).attr("title")).unix())+"s ago");
	});
}
