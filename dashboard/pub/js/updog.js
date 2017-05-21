//window.setInterval(updateApplications, 5000);
window.setInterval(updateTimestamps, 1000);

var jsonStream = new EventSource('/api/streaming/applications/')
jsonStream.onmessage = function (e) {
	var data = JSON.parse(e.data);
	//console.log(data);
	$.each(data.applications, function(an, app) {
		if ($('#app_'+an).length == 0) {
			$('#content').append('<div class="application" id="app_'+an+'"><div class="title">'+an+'</div></div>');
		}
		$.each(app.services, function(sn, serv) {
			if ($('#serv_'+an+'_'+sn).length == 0) {
				$('#app_'+an).append('<div class="service" id="serv_'+an+'_'+sn+'"><div class="serv_sum"><div class="title">'+sn+'</div></div></div>');
				$('#serv_'+an+'_'+sn+' .serv_sum').append('<div class="stat"></div><div class="nup"></div><div class="art"></div>');
				$('#serv_'+an+'_'+sn).append('<div class="inst_table"><hr /><table><thead><th class="inh">Instance Name</th><th class="rth">Response Time</th><th class="lch">Last Checked</th></thead><tbody></tbody></div>');
			}

			if (!serv.degraded && !serv.failed) {
				$('#serv_'+an+'_'+sn+' .stat').text("up");
				if(!$('#serv_'+an+'_'+sn+' .serv_sum').hasClass("up")) {
					$('#serv_'+an+'_'+sn).children('.inst_table').slideUp();
					$('#serv_'+an+'_'+sn+' .serv_sum').removeClass('degraded').removeClass('failed');
				}
				$('#serv_'+an+'_'+sn+' .serv_sum').addClass("up");
			}

			if (serv.degraded && !serv.failed) {
				$('#serv_'+an+'_'+sn+' .stat').text("degraded");
				if(!$('#serv_'+an+'_'+sn+' .serv_sum').hasClass("degraded")) {
					$('#serv_'+an+'_'+sn).children('.inst_table').slideDown();
					$('html, body').animate({
						scrollTop: ($('#app_'+an).offset().top)
					},500);
					$('#serv_'+an+'_'+sn+' .serv_sum').removeClass('up').removeClass('failed');
				}
				$('#serv_'+an+'_'+sn+' .serv_sum').addClass("degraded");
			}

			if (serv.failed) {
				$('#serv_'+an+'_'+sn+' .stat').text("failed");
				if(!$('#serv_'+an+'_'+sn+' .serv_sum').hasClass("failed")) {
					$('#serv_'+an+'_'+sn).children('.inst_table').slideDown();
					$('html, body').animate({
						scrollTop: ($('#app_'+an).offset().top)
					},500);
					$('#serv_'+an+'_'+sn+' .serv_sum').removeClass('up').removeClass('degraded');
				}
				$('#serv_'+an+'_'+sn+' .serv_sum').addClass("failed");
			}

			var tot_rt = 0

			$.each(serv.instances, function(iname, inst) {

				if ($(jq('inst_'+iname)).length == 0) {
					$('#serv_'+an+'_'+sn+' table tbody').append('<tr class="instance" id="inst_'+iname+'"></tr>');
				}
				if (inst.up) {
					$(jq('inst_'+iname)).removeClass("failed");
					$(jq('inst_'+iname)).addClass("up");
					tot_rt += inst.ResponseTime;
				} else {
					$(jq('inst_'+iname)).removeClass("up");
					$(jq('inst_'+iname)).addClass("failed");
				}

				$(jq('inst_'+iname)).html('<td class="ind">'+iname+'</td><td class="rtd">'+toMsFormatted(inst.response_time)+'</td><td class="lcd"><time title="'+inst.timestamp+'" ></time></td>');
			});

				if (serv.instances_up > 0) {
					$('#serv_'+an+'_'+sn+' .art').text(toMsFormatted(serv.average_response_time)+"ms avg");
				} else {
					$('#serv_'+an+'_'+sn+' .art').text("no response");
				}
				$('#serv_'+an+'_'+sn+' .nup').text(serv.instances_up+"/"+(serv.instances_total)+" up");
			});
		});

		updateTimestamps();

		if(data.failed) {
			$('.header').addClass("failed").removeClass('degraded').removeClass('up');
		} else if(data.degraded) {
			$('.header').addClass("degraded").removeClass('failed').removeClass('up');
		} else {
			$('.header').addClass("up").removeClass('failed').removeClass('degraded');
		}

		$('.aup').text(data.applications_up+'/'+data.applications_total+' apps');
		$('.sup').text(data.services_up+'/'+data.services_total+' services');
		$('.iup').text(data.instances_up+'/'+data.instances_total+' instances');
}

$(function() {
	//updateApplications();

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

function jq( myid ) {
    return "#" + myid.replace( /(\/|:|\.|\[|\]|,|=|@|\?)/g, "\\$1" );
}
