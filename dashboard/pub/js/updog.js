window.setInterval(updateApplications, 5000);
window.setInterval(updateTimestamps, 1000);

function updateApplications() {
	var api = '/api/applications'
	var apiParam = new RegExp('[\?&]api=([^&#]*)').exec(window.location.href)
	if (apiParam != null && apiParam[1]) {
		api = apiParam[1]
	}

	$.getJSON(api, function(data) {
		//console.log(data);
		$.each(data, function(an, app) {
			if ($('#app_'+an).length == 0) {
				$('#content').append('<div class="application" id="app_'+an+'"><div class="title">'+an+'</div></div>');
			}
			$.each(app.Services, function(sn, serv) {
				if ($('#serv_'+an+'_'+sn).length == 0) {
					$('#app_'+an).append('<div class="service" id="serv_'+an+'_'+sn+'"><div class="serv_sum"><div class="title">'+sn+'</div></div></div>');
					$('#serv_'+an+'_'+sn+' .serv_sum').append('<div class="stat"></div><div class="nup"></div><div class="art"></div>');
					$('#serv_'+an+'_'+sn).append('<div class="inst_table"><hr /><table><thead><th class="inh">Instance Name</th><th class="rth">Response Time</th><th class="lch">Last Checked</th></thead><tbody></tbody></div>');
				}

				if (!serv.IsDegraded && !serv.IsFailed) {
					$('#serv_'+an+'_'+sn+' .stat').text("up");
					if(!$('#serv_'+an+'_'+sn+' .serv_sum').hasClass("up")) {
						$('#serv_'+an+'_'+sn).children('.inst_table').slideUp();
						$('#serv_'+an+'_'+sn+' .serv_sum').removeClass('degraded').removeClass('failed');
					}
					$('#serv_'+an+'_'+sn+' .serv_sum').addClass("up");
				}

				if (serv.IsDegraded && !serv.IsFailed) {
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

				if (serv.IsFailed) {
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

				$.each(serv.Instances, function(iname, inst) {

					var fin = iname.replace(/\//g, "_").replace(/:/g, "_").replace(/\./g, "_");
					if ($('#inst_'+fin).length == 0) {
						$('#serv_'+an+'_'+sn+' table tbody').append('<tr class="instance" id="inst_'+fin+'"></tr>');
					}
					if (inst.Up) {
						$('#inst_'+fin).removeClass("failed");
						$('#inst_'+fin).addClass("up");
						tot_rt += inst.ResponseTime;
					} else {
						$('#inst_'+fin).removeClass("up");
						$('#inst_'+fin).addClass("failed");
					}

					$('#inst_'+fin).html('<td class="ind">'+iname+'</td><td class="rtd">'+toMsFormatted(inst.ResponseTime)+'</td><td class="lcd"><time title="'+inst.TimeStamp+'" ></time></td>');
				});

					if (serv.Up > 0) {
					var avg_rt = tot_rt / serv.Up;
						$('#serv_'+an+'_'+sn+' .art').text(toMsFormatted(avg_rt)+"ms avg");
					} else {
						$('#serv_'+an+'_'+sn+' .art').text("no response");
					}
					$('#serv_'+an+'_'+sn+' .nup').text(serv.Up+"/"+(serv.Down+serv.Up)+" up");
			});
		});

		updateTimestamps();

		if($('.serv_sum.failed').length > 0) {
			$('.header').addClass("failed").removeClass('degraded').removeClass('up');
		} else if($('.serv_sum.degraded').length > 0) {
			$('.header').addClass("degraded").removeClass('failed').removeClass('up');
		} else {
			$('.header').addClass("up").removeClass('failed').removeClass('degraded');
		}

		var apps = $('.application').length;
		var dapps = $('.application .serv_sum:not(.up):first-child').length;
		$('.aup').text((apps-dapps)+'/'+apps+' apps');
		$('.sup').text($('.serv_sum.up').length+'/'+$('.serv_sum').length+' services');
		$('.iup').text($('[id^=inst].up').length+'/'+$('[id^=inst]').length+' instances');
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
