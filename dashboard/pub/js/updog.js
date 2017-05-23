//window.setInterval(updateApplications, 5000);
//window.setInterval(updateTimestamps, 1000);

var jsonStream = new EventSource('/api/streaming/applications/')
jsonStream.onmessage = processMessage

var contentDiv = $('#content');
var timestampUpdaters = {};

function processMessage(e) {
	var data = JSON.parse(e.data);
	//console.log(data);
	$.each(data.applications, function(an, app) {
		var appDiv = $('#app_'+an)
		if (appDiv.length == 0) {
			contentDiv.append('<div class="application" id="app_'+an+'"><div class="title">'+an+'</div></div>');
			appDiv = $('#app_'+an)
		}
		$.each(app.services, function(sn, serv) {
			srvDiv = $('#serv_'+an+'_'+sn)
			if (srvDiv.length == 0) {
				appDiv.append('<div class="service" id="serv_'+an+'_'+sn+'"><div class="serv_sum"><div class="title">'+sn+'</div></div></div>');
				srvDiv = $('#serv_'+an+'_'+sn)
				srvDiv.find('.serv_sum').append('<div class="stat"></div><div class="nup"></div><div class="art"></div>');
				srvDiv.append('<div class="inst_table"><hr /><table><thead><th class="inh">Instance Name</th><th class="rth">Response Time</th><th class="lch">Last Checked</th></thead><tbody></tbody></div>');
			}
			var srvSum = srvDiv.find('.serv_sum')
			var srvStat = srvDiv.find('.stat')
			var srvTable = srvDiv.children('.inst_table')

			if (!serv.degraded && !serv.failed) {
				srvStat.text("up");
				if(!srvSum.hasClass("up")) {
					srvTable.slideUp();
					srvSum.removeClass('degraded').removeClass('failed');
				}
				srvSum.addClass("up");
			}

			if (serv.degraded && !serv.failed) {
				srvStat.text("degraded");
				if(!srvSum.hasClass("degraded")) {
					srvTable.slideDown();
					if (idleTime > 60 ) {
						$('html, body').animate({
							scrollTop: (appDiv.offset().top)
						},500);
					}
					srvSum.removeClass('up').removeClass('failed');
				}
				srvSum.addClass("degraded");
			}

			if (serv.failed) {
				srvStat.text("failed");
				if(!srvSum.hasClass("failed")) {
					srvTable.slideDown();
					if (idleTime > 60 ) {
						$('html, body').animate({
							scrollTop: (appDiv.offset().top)
						},500);
					}
					srvSum.removeClass('up').removeClass('degraded');
				}
				srvSum.addClass("failed");
			}

			var tot_rt = 0

			$.each(serv.instances, function(iname, inst) {
				var id = jq('inst_'+iname);

				var instDiv = $(id)
				if (instDiv.length == 0) {
					srvDiv.find('table tbody').append('<tr class="instance" id="inst_'+iname+'"></tr>');
					instDiv = $(id)
				}
				if (inst.up) {
					instDiv.removeClass("failed");
					instDiv.addClass("up");
					tot_rt += inst.ResponseTime;
				} else {
					instDiv.removeClass("up");
					instDiv.addClass("failed");
				}

				instDiv.html('<td class="ind">'+iname+'</td><td class="rtd">'+toMsFormatted(inst.response_time)+'</td><td class="lcd"><time title="'+inst.timestamp+'" ></time></td>');
				var instTime = instDiv.find('time');
				var timeStamp = moment(instTime.attr("title"))
				instTime.text((moment().unix() - timeStamp.unix())+"s ago");
				if (id in timestampUpdaters) {
					clearInterval(timestampUpdaters[id]);
				}
				setTimeout(function() {
					instTime.text((moment().unix() - timeStamp.unix())+"s ago");
					timestampUpdaters[id] = setInterval(function(){
						instTime.text((moment().unix() - timeStamp.unix())+"s ago");
					}, 1000);
				}, (moment.valueOf() - timeStamp.valueOf())%1000);
			});

				if (serv.instances_up > 0) {
					srvDiv.find('.art').text(toMsFormatted(serv.average_response_time)+"ms avg");
				} else {
					srvDiv.find('.art').text("no response");
				}
				srvDiv.find('.nup').text(serv.instances_up+"/"+(serv.instances_total)+" up");
			});
	});

	header = $('.header')
	if(data.failed) {
		header.addClass("failed").removeClass('degraded').removeClass('up');
	} else if(data.degraded) {
		header.addClass("degraded").removeClass('failed').removeClass('up');
	} else {
		header.addClass("up").removeClass('failed').removeClass('degraded');
	}

	$('.aup').text(data.applications_up+'/'+data.applications_total+' apps');
	$('.sup').text(data.services_up+'/'+data.services_total+' services');
	$('.iup').text(data.instances_up+'/'+data.instances_total+' instances');
}

$(function() {
	contentDiv = $('#content');
	contentDiv.on("click", '.service', function(event) {
		$(event.target).children('div.inst_table').slideToggle();
	});

	contentDiv.on("click", '.serv_sum', function(event) {
		$(event.target).parent().trigger("click");
	});
	
    //Increment the idle time counter every second.
    var idleInterval = setInterval(timerIncrement, 1000);

    //Zero the idle timer on mouse movement.
    $(this).mousemove(function (e) {
        idleTime = 0;
    });
    $(this).keypress(function (e) {
        idleTime = 0;
    });
});

function toMsFormatted(number) {
	return (Math.round(number/10000)/100).toFixed(2);
}

function updateTimestamps() {
	var now = moment().unix()
	$('time').each(function() {
		$(this).text((now - moment($(this).attr("title")).unix())+"s ago");
	});
}

function jq( myid ) {
    return "#" + myid.replace( /(\/|:|\.|\[|\]|,|=|@|\?)/g, "\\$1" );
}
